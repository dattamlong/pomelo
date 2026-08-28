package renderflow

import (
	"testing"
	"time"
)

func fixedStore(at time.Time) *Store {
	s := NewStore()
	s.now = func() time.Time { return at }
	return s
}

func TestSummaryAggregatesAndFlags(t *testing.T) {
	now := time.UnixMilli(1_000_000)
	s := fixedStore(now)
	ms := now.UnixMilli()
	s.Ingest("feat-x", "client", "portal", Batch{Commits: []Commit{
		{T: ms - 1000, Dur: 30, Renders: []Render{
			{Name: "LeadTable", Self: 20, Why: "parent", Wasted: true, Count: 1},
			{Name: "LeadRow", Self: 8, Why: "props:lead", Count: 40},
		}},
		{T: ms - 500, Dur: 5, Renders: []Render{
			{Name: "LeadTable", Self: 2, Why: "state", Count: 1},
		}},
		{T: ms - 60_000, Dur: 99, Renders: []Render{{Name: "Old", Self: 99, Count: 1}}},
	}})
	sum := s.Summary("feat-x", 10*time.Second, DefaultThresholds)
	if len(sum.Targets) != 1 || sum.Targets[0].Commits != 2 {
		t.Fatalf("targets/commits: %+v", sum.Targets)
	}
	comps := sum.Targets[0].Components
	if len(comps) != 2 {
		t.Fatalf("want 2 components (Old outside window), got %d", len(comps))
	}
	lt := comps[0]
	if lt.Name != "LeadTable" || lt.Renders != 2 || lt.Wasted != 1 || lt.SelfMax != 20 || lt.SelfAvg != 11 {
		t.Fatalf("LeadTable: %+v", lt)
	}
	if !has(lt.Flags, "slow") || !has(lt.Flags, "wasted") || has(lt.Flags, "hot") {
		t.Fatalf("LeadTable flags: %v", lt.Flags)
	}
	if lt.Why["parent"] != 1 || lt.Why["state"] != 1 {
		t.Fatalf("LeadTable why: %v", lt.Why)
	}
	lr := comps[1]
	if lr.Renders != 40 || !has(lr.Flags, "hot") || lr.Why["props"] != 40 || lr.SelfMax != 0.2 {
		t.Fatalf("LeadRow: %+v", lr)
	}
}

func TestSummaryIsolatesBranchesAndEmitsEmptySlices(t *testing.T) {
	s := fixedStore(time.UnixMilli(5000))
	s.Ingest("a", "r", "web", Batch{Commits: []Commit{{T: 4000, Renders: []Render{{Name: "X", Count: 1}}}}})
	if got := s.Summary("b", 0, DefaultThresholds); got.Targets == nil || len(got.Targets) != 0 {
		t.Fatalf("want empty non-nil targets, got %#v", got.Targets)
	}
	got := s.Summary("a", 0, DefaultThresholds)
	if got.WindowS != 10 || len(got.Targets) != 1 || got.Targets[0].Components[0].Flags == nil {
		t.Fatalf("summary: %+v", got)
	}
	s.Clear("a")
	if len(s.Summary("a", 0, DefaultThresholds).Targets) != 0 {
		t.Fatal("clear did not drop the branch")
	}
}

func TestRingCap(t *testing.T) {
	s := fixedStore(time.UnixMilli(1))
	cs := make([]Commit, ringCap+10)
	s.Ingest("a", "r", "w", Batch{Commits: cs})
	if n := len(s.targets["a\x00r/w"].commits); n != ringCap {
		t.Fatalf("ring = %d", n)
	}
}

func TestIsVendor(t *testing.T) {
	cases := map[string]bool{"LeadTable": false, "Card": false, "Show": false, "Anonymous": false,
		"te$1": true, "eo": true, "O0": true, "s$6": true, "i2": true, "Svg": false}
	for n, want := range cases {
		if got := isVendor(n, nil); got != want {
			t.Errorf("%q vendor=%v want %v", n, got, want)
		}
	}
	if !isVendor("Button", &Src{File: "/x/node_modules/lib/Button.js"}) || isVendor("te$1", &Src{File: "/x/src/a.tsx"}) {
		t.Error("src path must decide when present")
	}
}

func has(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

func TestGraphScopesToChangedComponentsAndLayers(t *testing.T) {
	now := time.UnixMilli(1_000_000)
	s := fixedStore(now)
	ms := now.UnixMilli()
	src := func(f string) *Src { return &Src{File: f, Line: 1} }
	s.Ingest("feat", "client", "portal", Batch{Commits: []Commit{{T: ms - 100, Trigger: "click \"Save\"", Renders: []Render{
		{I: 0, P: -1, Name: "Page", Self: 1, Why: "state", Src: src("/wt/src/Page.tsx")},
		{I: 1, P: 0, Name: "Card", Self: 0.2, Why: "parent", Wasted: true, Src: src("/wt/src/Card.tsx")},
		{I: 2, P: 1, Name: "te$1", Self: 0, Why: "parent", Wasted: true},
		{I: 3, P: 1, Name: "Switch", Self: 0.1, Why: "props:checked", Src: src("/wt/src/Switch.tsx")},
		{I: 4, P: 0, Name: "Sidebar", Self: 0.3, Why: "parent", Wasted: true, Src: src("/wt/src/Sidebar.tsx")},
		{I: 5, P: 4, Name: "NavLink", Self: 0, Why: "parent", Wasted: true, Src: src("/wt/src/NavLink.tsx")},
	}}}})
	changed := func(x *Src) bool { return x.File == "/wt/src/Card.tsx" }
	g := s.Graph("feat", "client", "portal", 10*time.Second, DefaultThresholds, changed)
	if !g.Scoped || g.Commits != 1 || len(g.Triggers) != 1 {
		t.Fatalf("graph meta: %+v", g)
	}
	names := map[string]GraphNode{}
	for _, n := range g.Nodes {
		names[n.Name] = n
	}
	if _, ok := names["Sidebar"]; ok {
		t.Fatal("Sidebar is unrelated to the change and must be dropped")
	}
	if _, ok := names["te$1"]; ok {
		t.Fatal("vendor neighbour must be dropped")
	}
	if !names["Card"].Changed || names["Page"].Changed || names["Switch"].Changed {
		t.Fatalf("changed flags: %+v", g.Nodes)
	}
	if names["Page"].Depth != 0 || names["Card"].Depth != 1 || names["Switch"].Depth != 2 {
		t.Fatalf("depths: %+v", g.Nodes)
	}
	if len(g.Edges) != 2 || g.Edges[0].From != "Card" || g.Edges[1].From != "Page" {
		t.Fatalf("edges: %+v", g.Edges)
	}
	all := s.Graph("feat", "client", "portal", 10*time.Second, DefaultThresholds, func(*Src) bool { return false })
	if all.Scoped || len(all.Nodes) != 5 {
		t.Fatalf("unscoped fallback should keep every non-vendor node: %+v", all.Nodes)
	}
	if empty := s.Graph("other", "client", "portal", 0, DefaultThresholds, nil); empty.Nodes == nil || empty.Edges == nil || empty.Triggers == nil {
		t.Fatal("empty graph must marshal arrays, not null")
	}
}
