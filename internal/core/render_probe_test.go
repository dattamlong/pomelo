package core

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
	cfg := &config.Config{Repos: map[string]*config.Dir{
		"boompay-client": {Alias: "client", Services: map[string]*config.Service{"portal": {RenderProbe: true}}},
	}}
	if svc := serviceConfig(cfg, "client/portal"); svc == nil || !svc.RenderProbe {
		t.Fatal("alias lookup failed")
	}
	if serviceConfig(cfg, "boompay-client/portal") == nil || serviceConfig(cfg, "client/nope") != nil || serviceConfig(cfg, "client") != nil {
		t.Fatal("name/miss lookup wrong")
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
