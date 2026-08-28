package core

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/pomelohq/pomelo/internal/config"
	"github.com/pomelohq/pomelo/internal/renderflow"
)

// Reserved dev-proxy namespace: never forwarded upstream.
const renderProbePrefix = "/_pom_dev/_pom/"

const maxProbeBody = 256 << 10

func (s *Server) renderProbeEnabled(target string) bool {
	svc := serviceConfig(s.cfg(), target)
	return svc != nil && svc.RenderProbe
}

func serviceConfig(cfg *config.Config, target string) *config.Service {
	if cfg == nil {
		return nil
	}
	i := strings.IndexByte(target, '/')
	if i < 0 {
		return nil
	}
	alias, svc := target[:i], target[i+1:]
	for name, d := range cfg.Repos {
		a := d.Alias
		if a == "" {
			a = name
		}
		if a == alias || name == alias {
			return d.Services[svc]
		}
	}
	return nil
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
