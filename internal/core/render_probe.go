package core

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pomelohq/pomelo/internal/config"
	"github.com/pomelohq/pomelo/internal/renderflow"
	"github.com/pomelohq/pomelo/internal/services"
)

// Reserved dev-proxy namespace: never forwarded upstream.
const renderProbePrefix = "/_pom_dev/_pom/"

const maxProbeBody = 256 << 10

// Precedence: manual toggle from the app > pom.yml render_probe > auto (React
// in the service dir's package.json).
func (s *Server) renderProbeEnabled(branch, target string) bool {
	if branch == "" {
		return false
	}
	s.probeMu.Lock()
	on, manual := s.probeOn[branch+"\x00"+target]
	s.probeMu.Unlock()
	if manual {
		return on
	}
	repo, svc := serviceConfig(s.cfg(), target)
	if svc == nil {
		return false
	}
	if svc.RenderProbe != nil {
		return *svc.RenderProbe
	}
	return detectReact(s.serviceDir(branch, repo, svc))
}

func (s *Server) serviceDir(branch, repo string, svc *config.Service) string {
	wt := services.RepoWorktreePath(s.WorkspaceRoot, repo, branch, branch == s.DefaultBranch)
	if svc.Dir != "" {
		return filepath.Join(wt, svc.Dir)
	}
	return wt
}

var reactCache sync.Map // package.json path -> reactProbe

type reactProbe struct {
	mod   time.Time
	react bool
}

func detectReact(dir string) bool {
	pj := filepath.Join(dir, "package.json")
	st, err := os.Stat(pj)
	if err != nil {
		return false
	}
	if c, ok := reactCache.Load(pj); ok && c.(reactProbe).mod.Equal(st.ModTime()) {
		return c.(reactProbe).react
	}
	raw, err := os.ReadFile(pj)
	react := false
	if err == nil {
		var pkg struct {
			Deps    map[string]string `json:"dependencies"`
			DevDeps map[string]string `json:"devDependencies"`
		}
		if json.Unmarshal(raw, &pkg) == nil {
			_, a := pkg.Deps["react"]
			_, b := pkg.DevDeps["react"]
			react = a || b
		}
	}
	reactCache.Store(pj, reactProbe{mod: st.ModTime(), react: react})
	return react
}

func serviceConfig(cfg *config.Config, target string) (repo string, svc *config.Service) {
	if cfg == nil {
		return "", nil
	}
	i := strings.IndexByte(target, '/')
	if i < 0 {
		return "", nil
	}
	alias, name := target[:i], target[i+1:]
	for rname, d := range cfg.Repos {
		a := d.Alias
		if a == "" {
			a = rname
		}
		if a == alias || rname == alias {
			return rname, d.Services[name]
		}
	}
	return "", nil
}

type ProbeInfo struct {
	Repo    string `json:"repo"`
	Svc     string `json:"svc"`
	Target  string `json:"target"`
	React   bool   `json:"react"`
	Enabled bool   `json:"enabled"`
	Source  string `json:"source"` // manual | config | auto
}

// RenderProbes lists every ported service of the branch with its probe state so
// the app can offer a toggle without knowing the precedence rules.
func (s *Server) RenderProbes(branch string) map[string]any {
	out := []ProbeInfo{}
	cfg := s.cfg()
	if cfg == nil || branch == "" {
		return map[string]any{"probes": out}
	}
	for rname, d := range cfg.Repos {
		alias := d.Alias
		if alias == "" {
			alias = rname
		}
		for name, svc := range d.Services {
			if svc == nil || svc.Port == nil || !*svc.Port {
				continue
			}
			target := alias + "/" + name
			p := ProbeInfo{Repo: alias, Svc: name, Target: target, React: detectReact(s.serviceDir(branch, rname, svc))}
			s.probeMu.Lock()
			on, manual := s.probeOn[branch+"\x00"+target]
			s.probeMu.Unlock()
			switch {
			case manual:
				p.Enabled, p.Source = on, "manual"
			case svc.RenderProbe != nil:
				p.Enabled, p.Source = *svc.RenderProbe, "config"
			default:
				p.Enabled, p.Source = p.React, "auto"
			}
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Target < out[j].Target })
	return map[string]any{"probes": out}
}

func (s *Server) RenderSetProbe(branch, target string, enabled bool) map[string]any {
	if branch == "" || !strings.Contains(target, "/") {
		return map[string]any{"ok": false, "error": "branch and target required"}
	}
	s.probeMu.Lock()
	if s.probeOn == nil {
		s.probeOn = map[string]bool{}
	}
	s.probeOn[branch+"\x00"+target] = enabled
	s.probeMu.Unlock()
	return map[string]any{"ok": true}
}

func (s *Server) handleRenderProbe(w http.ResponseWriter, r *http.Request, branchLabel string) {
	switch strings.TrimPrefix(r.URL.Path, renderProbePrefix) {
	case "probe.js":
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(renderflow.ProbeJS)
	case "render":
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		target := r.URL.Query().Get("target")
		i := strings.IndexByte(target, '/')
		branch := s.branchForHostLabel(branchLabel)
		if i < 0 || branch == "" {
			http.Error(w, "unknown probe target", http.StatusBadRequest)
			return
		}
		var b renderflow.Batch
		if err := json.NewDecoder(io.LimitReader(r.Body, maxProbeBody)).Decode(&b); err != nil {
			http.Error(w, "bad probe batch: "+err.Error(), http.StatusBadRequest)
			return
		}
		s.renderFlow.Ingest(branch, target[:i], target[i+1:], b)
		w.WriteHeader(http.StatusNoContent)
	default:
		http.NotFound(w, r)
	}
}

func wantsHTML(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept"), "text/html")
}

// Splices the probe <script> right after <head> so it installs before the app
// bundle loads; leaves non-HTML and already-encoded responses untouched.
func injectRenderProbe(resp *http.Response, target string) error {
	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/html") || resp.Header.Get("Content-Encoding") != "" || resp.Body == nil {
		return nil
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return err
	}
	tag := []byte(`<script src="` + renderProbePrefix + `probe.js" data-target="` + target + `"></script>`)
	out := spliceAfterHead(body, tag)
	resp.Body = io.NopCloser(bytes.NewReader(out))
	resp.ContentLength = int64(len(out))
	resp.Header.Set("Content-Length", strconv.Itoa(len(out)))
	return nil
}

func spliceAfterHead(body, tag []byte) []byte {
	limit := len(body)
	if limit > 64<<10 {
		limit = 64 << 10
	}
	lower := bytes.ToLower(body[:limit])
	i := bytes.Index(lower, []byte("<head"))
	if i < 0 {
		return append(append(tag, '\n'), body...)
	}
	j := bytes.IndexByte(lower[i:], '>')
	if j < 0 {
		return append(append(tag, '\n'), body...)
	}
	at := i + j + 1
	out := make([]byte, 0, len(body)+len(tag))
	out = append(out, body[:at]...)
	out = append(out, tag...)
	return append(out, body[at:]...)
}

func (s *Server) RenderSummary(branch string, windowS int) renderflow.Summary {
	return s.renderFlow.Summary(branch, time.Duration(windowS)*time.Second, renderflow.DefaultThresholds)
}

func (s *Server) RenderClear(branch string) map[string]any {
	s.renderFlow.Clear(branch)
	return map[string]any{"ok": true}
}
