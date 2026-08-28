package core

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pomelohq/pomelo/internal/config"
	"github.com/pomelohq/pomelo/internal/renderflow"
)

func TestSpliceAfterHeadInsertsOnceAfterOpeningTag(t *testing.T) {
	got := string(spliceAfterHead([]byte("<!doctype html><HTML><Head lang=\"en\"><title>x</title></head>"), []byte("<script></script>")))
	want := "<!doctype html><HTML><Head lang=\"en\"><script></script><title>x</title></head>"
	if got != want {
		t.Fatalf("got %q", got)
	}
	if got := string(spliceAfterHead([]byte("<div>no head</div>"), []byte("T"))); got != "T\n<div>no head</div>" {
		t.Fatalf("no-head fallback: %q", got)
	}
}

func TestInjectRenderProbeOnlyTouchesPlainHTML(t *testing.T) {
	mk := func(ct, enc, body string) *http.Response {
		h := http.Header{"Content-Type": {ct}}
		if enc != "" {
			h.Set("Content-Encoding", enc)
		}
		return &http.Response{Header: h, Body: io.NopCloser(strings.NewReader(body)), ContentLength: int64(len(body))}
	}
	r := mk("text/html; charset=utf-8", "", "<head></head>")
	if err := injectRenderProbe(r, "client/portal"); err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(r.Body)
	if !strings.Contains(string(b), `data-target="client/portal"`) || r.ContentLength != int64(len(b)) {
		t.Fatalf("html not injected: %q len=%d", b, r.ContentLength)
	}
	for _, r := range []*http.Response{mk("application/json", "", "{}"), mk("text/html", "gzip", "zz")} {
		before, _ := io.ReadAll(r.Body)
		r.Body = io.NopCloser(strings.NewReader(string(before)))
		_ = injectRenderProbe(r, "x/y")
		after, _ := io.ReadAll(r.Body)
		if string(before) != string(after) {
			t.Fatalf("touched %s: %q", r.Header.Get("Content-Type"), after)
		}
	}
}

func TestServiceConfigResolvesAliasOrName(t *testing.T) {
	on := true
	cfg := &config.Config{Repos: map[string]*config.Dir{
		"boompay-client": {Alias: "client", Services: map[string]*config.Service{"portal": {RenderProbe: &on}}},
	}}
	if repo, svc := serviceConfig(cfg, "client/portal"); repo != "boompay-client" || svc == nil || svc.RenderProbe == nil || !*svc.RenderProbe {
		t.Fatal("alias lookup failed")
	}
	if _, svc := serviceConfig(cfg, "boompay-client/portal"); svc == nil {
		t.Fatal("name lookup failed")
	}
	if _, svc := serviceConfig(cfg, "client/nope"); svc != nil {
		t.Fatal("miss should be nil")
	}
	if _, svc := serviceConfig(cfg, "client"); svc != nil {
		t.Fatal("no slash should be nil")
	}
}

func TestDetectReactReadsPackageJSON(t *testing.T) {
	dir := t.TempDir()
	if detectReact(dir) {
		t.Fatal("no package.json must be false")
	}
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"dependencies":{"vue":"3"}}`), 0o644)
	if detectReact(dir) {
		t.Fatal("vue app must be false")
	}
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"devDependencies":{"react":"^18"}}`), 0o644)
	future := time.Now().Add(2 * time.Second)
	os.Chtimes(filepath.Join(dir, "package.json"), future, future)
	if !detectReact(dir) {
		t.Fatal("react in devDependencies must be true")
	}
}

func TestRenderProbePrecedence(t *testing.T) {
	root := t.TempDir()
	on, off := true, false
	port := true
	svcDir := filepath.Join(root, "workspace--feat", "client", "apps", "web")
	os.MkdirAll(svcDir, 0o755)
	os.WriteFile(filepath.Join(svcDir, "package.json"), []byte(`{"dependencies":{"react":"18"}}`), 0o644)
	s := &Server{WorkspaceRoot: root, DefaultBranch: "main", renderFlow: renderflow.NewStore()}
	s.cfgv.Store(&config.Config{Repos: map[string]*config.Dir{
		"client": {Services: map[string]*config.Service{
			"web":   {Dir: "apps/web", Port: &port},
			"api":   {Dir: "apps/api", Port: &port},
			"force": {Dir: "apps/api", Port: &port, RenderProbe: &on},
			"never": {Dir: "apps/web", Port: &port, RenderProbe: &off},
		}},
	}})
	if !s.renderProbeEnabled("feat", "client/web") {
		t.Fatal("auto-detect should enable react service")
	}
	if s.renderProbeEnabled("feat", "client/api") {
		t.Fatal("non-react service should be off")
	}
	if !s.renderProbeEnabled("feat", "client/force") || s.renderProbeEnabled("feat", "client/never") {
		t.Fatal("config should override auto")
	}
	s.RenderSetProbe("feat", "client/never", true)
	if !s.renderProbeEnabled("feat", "client/never") {
		t.Fatal("manual toggle should override config")
	}
	probes := s.RenderProbes("feat")["probes"].([]ProbeInfo)
	if len(probes) != 4 || probes[0].Target != "client/api" || probes[0].Enabled || probes[1].Source != "config" || probes[2].Source != "manual" || !probes[3].React {
		t.Fatalf("probes: %+v", probes)
	}
}

func TestHandleRenderProbeServesScriptAndIngests(t *testing.T) {
	s := &Server{renderFlow: renderflow.NewStore()}
	s.cfgv.Store(&config.Config{})
	rec := httptest.NewRecorder()
	s.handleRenderProbe(rec, httptest.NewRequest("GET", renderProbePrefix+"probe.js", nil), "feat")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "__REACT_DEVTOOLS_GLOBAL_HOOK__") {
		t.Fatalf("probe.js: %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	s.handleRenderProbe(rec, httptest.NewRequest("GET", renderProbePrefix+"render?target=a/b", nil), "feat")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET render: %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	s.handleRenderProbe(rec, httptest.NewRequest("POST", renderProbePrefix+"render?target=nope", strings.NewReader("{}")), "feat")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad target: %d", rec.Code)
	}
}

func TestRenderGraphUnknownTargetIsEmptyNotNull(t *testing.T) {
	s := &Server{WorkspaceRoot: t.TempDir(), DefaultBranch: "main", renderFlow: renderflow.NewStore()}
	s.cfgv.Store(&config.Config{})
	g := s.RenderGraph("feat", "nope", 10)
	if g.Nodes == nil || g.Edges == nil || g.Triggers == nil || len(g.Nodes) != 0 {
		t.Fatalf("graph: %+v", g)
	}
	if files := s.changedFiles("main", "r"); files == nil || len(files) != 0 {
		t.Fatalf("main has no diff: %v", files)
	}
}
