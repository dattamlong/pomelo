// Package renderflow aggregates React render telemetry sent by the browser probe.
// Classification (hot / slow / wasted) is domain logic and lives here (ADR 0001).
package renderflow

import (
	_ "embed"
	"sort"
	"strings"
	"sync"
	"time"
)

//go:embed probe.js
var ProbeJS []byte

type Src struct {
	File string `json:"file"`
	Line int    `json:"line"`
}

type Render struct {
	I      int     `json:"i"`
	P      int     `json:"p"`
	Name   string  `json:"name"`
	Self   float64 `json:"self"`
	Why    string  `json:"why"`
	Wasted bool    `json:"wasted"`
	Count  int     `json:"count"`
	Src    *Src    `json:"src,omitempty"`
}

type Commit struct {
	T       int64    `json:"t"`
	Dur     float64  `json:"dur"`
	Trigger string   `json:"trigger"`
	Path    string   `json:"path"`
	Renders []Render `json:"renders"`
}

type Batch struct {
	Commits   []Commit `json:"commits"`
	ProbeMs   float64  `json:"probe_ms"`
	Truncated bool     `json:"truncated"`
}

type Thresholds struct {
	SlowMs    float64
	HotPer10s int
}

var DefaultThresholds = Thresholds{SlowMs: 16, HotPer10s: 20}

const ringCap = 2000

type target struct {
	repo, svc string
	commits   []Commit
	probeMs   float64
	truncated bool
	lastSeen  time.Time
}

type Store struct {
	mu      sync.Mutex
	targets map[string]*target // branch\x00repo/svc
	now     func() time.Time
}

func NewStore() *Store { return &Store{targets: map[string]*target{}, now: time.Now} }

func (s *Store) Ingest(branch, repo, svc string, b Batch) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := branch + "\x00" + repo + "/" + svc
	t := s.targets[key]
	if t == nil {
		t = &target{repo: repo, svc: svc}
		s.targets[key] = t
	}
	t.commits = append(t.commits, b.Commits...)
	if n := len(t.commits) - ringCap; n > 0 {
		t.commits = t.commits[n:]
	}
	t.probeMs += b.ProbeMs
	t.truncated = t.truncated || b.Truncated
	t.lastSeen = s.now()
}

func (s *Store) Clear(branch string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k := range s.targets {
		if len(k) > len(branch) && k[:len(branch)+1] == branch+"\x00" {
			delete(s.targets, k)
		}
	}
}

type Component struct {
	Name     string         `json:"name"`
	Vendor   bool           `json:"vendor"`
	Renders  int            `json:"renders"`
	Wasted   int            `json:"wasted"`
	SelfAvg  float64        `json:"self_avg"`
	SelfMax  float64        `json:"self_max"`
	Why      map[string]int `json:"why"`
	Flags    []string       `json:"flags"`
	Src      *Src           `json:"src,omitempty"`
	LastSeen int64          `json:"last_seen"`
}

type Target struct {
	Repo       string      `json:"repo"`
	Svc        string      `json:"svc"`
	Commits    int         `json:"commits"`
	Truncated  bool        `json:"truncated"`
	ProbeMs    float64     `json:"probe_ms"`
	LastSeen   int64       `json:"last_seen"`
	Components []Component `json:"components"`
}

type Summary struct {
	WindowS int      `json:"window_s"`
	Targets []Target `json:"targets"`
}

// Summary aggregates the commits of the last window seconds for every probed
// service of the branch. Components sort by wasted renders, then renders.
func (s *Store) Summary(branch string, window time.Duration, thr Thresholds) Summary {
	s.mu.Lock()
	defer s.mu.Unlock()
	if window <= 0 {
		window = 10 * time.Second
	}
	since := s.now().Add(-window).UnixMilli()
	out := Summary{WindowS: int(window / time.Second), Targets: []Target{}}
	for k, t := range s.targets {
		if len(k) <= len(branch) || k[:len(branch)+1] != branch+"\x00" {
			continue
		}
		out.Targets = append(out.Targets, t.summary(since, window, thr))
	}
	sort.Slice(out.Targets, func(i, j int) bool {
		if out.Targets[i].Repo != out.Targets[j].Repo {
			return out.Targets[i].Repo < out.Targets[j].Repo
		}
		return out.Targets[i].Svc < out.Targets[j].Svc
	})
	return out
}

func (t *target) summary(since int64, window time.Duration, thr Thresholds) Target {
	type acc struct {
		c       Component
		selfSum float64
	}
	byName := map[string]*acc{}
	commits := 0
	for _, c := range t.commits {
		if c.T < since {
			continue
		}
		commits++
		for _, r := range c.Renders {
			a := byName[r.Name]
			if a == nil {
				a = &acc{c: Component{Name: r.Name, Why: map[string]int{}, Flags: []string{}}}
				byName[r.Name] = a
			}
			n := r.Count
			if n <= 0 {
				n = 1
			}
			a.c.Renders += n
			if r.Wasted {
				a.c.Wasted += n
			}
			a.c.Why[whyKind(r.Why)] += n
			a.selfSum += r.Self
			per := r.Self / float64(n)
			if per > a.c.SelfMax {
				a.c.SelfMax = per
			}
			if r.Src != nil && a.c.Src == nil {
				a.c.Src = r.Src
			}
			if c.T > a.c.LastSeen {
				a.c.LastSeen = c.T
			}
		}
	}
	hotN := int(float64(thr.HotPer10s) * window.Seconds() / 10)
	comps := make([]Component, 0, len(byName))
	for _, a := range byName {
		c := a.c
		if c.Renders > 0 {
			c.SelfAvg = round2(a.selfSum / float64(c.Renders))
		}
		c.SelfMax = round2(c.SelfMax)
		c.Vendor = isVendor(c.Name, c.Src)
		if c.SelfMax >= thr.SlowMs {
			c.Flags = append(c.Flags, "slow")
		}
		if hotN > 0 && c.Renders >= hotN {
			c.Flags = append(c.Flags, "hot")
		}
		if c.Wasted > 0 && c.Wasted*2 >= c.Renders {
			c.Flags = append(c.Flags, "wasted")
		}
		comps = append(comps, c)
	}
	sort.Slice(comps, func(i, j int) bool {
		if comps[i].Wasted != comps[j].Wasted {
			return comps[i].Wasted > comps[j].Wasted
		}
		if comps[i].Renders != comps[j].Renders {
			return comps[i].Renders > comps[j].Renders
		}
		return comps[i].Name < comps[j].Name
	})
	return Target{Repo: t.repo, Svc: t.svc, Commits: commits, Truncated: t.truncated, ProbeMs: round2(t.probeMs),
		LastSeen: t.lastSeen.UnixMilli(), Components: comps}
}

// Library code: pre-bundled deps report node_modules paths or minified names
// (te$1, eo, O0). Users cannot fix those, so the UI hides them by default.
func isVendor(name string, src *Src) bool {
	if src != nil && src.File != "" {
		return strings.Contains(src.File, "/node_modules/")
	}
	if name == "" || name == "Anonymous" {
		return false
	}
	if strings.ContainsAny(name, "$") {
		return true
	}
	if len(name) <= 2 {
		return true
	}
	if len(name) <= 4 && strings.ContainsAny(name, "0123456789") {
		return true
	}
	return false
}

// "props:a,b" collapses to "props" so the histogram stays small.
func whyKind(w string) string {
	for i := 0; i < len(w); i++ {
		if w[i] == ':' {
			return w[:i]
		}
	}
	if w == "" {
		return "unknown"
	}
	return w
}

func round2(f float64) float64 { return float64(int64(f*100+0.5)) / 100 }

type GraphNode struct {
	Name     string         `json:"name"`
	Renders  int            `json:"renders"`
	Wasted   int            `json:"wasted"`
	SelfAvg  float64        `json:"self_avg"`
	SelfMax  float64        `json:"self_max"`
	SelfSum  float64        `json:"self_sum"`
	Why      map[string]int `json:"why"`
	Flags    []string       `json:"flags"`
	Src      *Src           `json:"src,omitempty"`
	Changed  bool           `json:"changed"`
	Vendor   bool           `json:"vendor"`
	Depth    int            `json:"depth"`
	LastSeen int64          `json:"last_seen"`
}

type GraphEdge struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Count int    `json:"count"`
}

type Graph struct {
	Repo     string      `json:"repo"`
	Svc      string      `json:"svc"`
	WindowS  int         `json:"window_s"`
	Commits  int         `json:"commits"`
	Scoped   bool        `json:"scoped"`
	Nodes    []GraphNode `json:"nodes"`
	Edges    []GraphEdge `json:"edges"`
	Triggers []string    `json:"triggers"`
}

// Graph is the render-propagation graph of the window: nodes are components,
// an edge parent->child means the child rendered inside the parent's render.
// When changed says which sources the branch touched, the graph is scoped to
// those components plus their direct neighbours; otherwise it shows everything
// that is not library code.
func (s *Store) Graph(branch, repo, svc string, window time.Duration, thr Thresholds, changed func(*Src) bool) Graph {
	s.mu.Lock()
	defer s.mu.Unlock()
	if window <= 0 {
		window = 10 * time.Second
	}
	g := Graph{Repo: repo, Svc: svc, WindowS: int(window / time.Second), Nodes: []GraphNode{}, Edges: []GraphEdge{}, Triggers: []string{}}
	t := s.targets[branch+"\x00"+repo+"/"+svc]
	if t == nil {
		return g
	}
	since := s.now().Add(-window).UnixMilli()
	nodes := map[string]*GraphNode{}
	edges := map[[2]string]int{}
	seenTrig := map[string]bool{}
	for _, c := range t.commits {
		if c.T < since {
			continue
		}
		g.Commits++
		if c.Trigger != "" && !seenTrig[c.Trigger] && len(g.Triggers) < 8 {
			seenTrig[c.Trigger] = true
			g.Triggers = append(g.Triggers, c.Trigger)
		}
		for _, r := range c.Renders {
			n := nodes[r.Name]
			if n == nil {
				n = &GraphNode{Name: r.Name, Why: map[string]int{}, Flags: []string{}}
				nodes[r.Name] = n
			}
			n.Renders++
			if r.Wasted {
				n.Wasted++
			}
			n.Why[whyKind(r.Why)]++
			n.SelfSum += r.Self
			if r.Self > n.SelfMax {
				n.SelfMax = r.Self
			}
			if r.Src != nil && n.Src == nil {
				n.Src = r.Src
			}
			if c.T > n.LastSeen {
				n.LastSeen = c.T
			}
			if r.P >= 0 && r.P < len(c.Renders) {
				par := c.Renders[r.P].Name
				if par != r.Name {
					edges[[2]string{par, r.Name}]++
				}
			}
		}
	}
	keep := map[string]bool{}
	anyChanged := false
	for name, n := range nodes {
		n.Vendor = isVendor(n.Name, n.Src)
		if changed != nil && n.Src != nil && changed(n.Src) {
			n.Changed = true
			anyChanged = true
			keep[name] = true
		}
	}
	g.Scoped = anyChanged
	if anyChanged {
		for e := range edges {
			if keep[e[0]] && nodes[e[1]] != nil && !nodes[e[1]].Vendor {
				keep[e[1]] = true
			}
			if keep[e[1]] && nodes[e[0]] != nil && !nodes[e[0]].Vendor {
				keep[e[0]] = true
			}
		}
	} else {
		for name, n := range nodes {
			if !n.Vendor {
				keep[name] = true
			}
		}
	}
	// Changed nodes always survive the neighbour pass; re-add in case a vendor
	// parent sat between two changed components.
	for name, n := range nodes {
		if n.Changed {
			keep[name] = true
		}
	}
	hotN := int(float64(thr.HotPer10s) * window.Seconds() / 10)
	for name, n := range nodes {
		if !keep[name] {
			continue
		}
		if n.Renders > 0 {
			n.SelfAvg = round2(n.SelfSum / float64(n.Renders))
		}
		n.SelfSum, n.SelfMax = round2(n.SelfSum), round2(n.SelfMax)
		if n.SelfMax >= thr.SlowMs {
			n.Flags = append(n.Flags, "slow")
		}
		if hotN > 0 && n.Renders >= hotN {
			n.Flags = append(n.Flags, "hot")
		}
		if n.Wasted > 0 && n.Wasted*2 >= n.Renders {
			n.Flags = append(n.Flags, "wasted")
		}
	}
	for e, cnt := range edges {
		if keep[e[0]] && keep[e[1]] {
			g.Edges = append(g.Edges, GraphEdge{From: e[0], To: e[1], Count: cnt})
		}
	}
	sort.Slice(g.Edges, func(i, j int) bool {
		if g.Edges[i].From != g.Edges[j].From {
			return g.Edges[i].From < g.Edges[j].From
		}
		return g.Edges[i].To < g.Edges[j].To
	})
	assignDepth(nodes, keep, g.Edges)
	for name, n := range nodes {
		if keep[name] {
			g.Nodes = append(g.Nodes, *n)
		}
	}
	sort.Slice(g.Nodes, func(i, j int) bool {
		if g.Nodes[i].Depth != g.Nodes[j].Depth {
			return g.Nodes[i].Depth < g.Nodes[j].Depth
		}
		if g.Nodes[i].Renders != g.Nodes[j].Renders {
			return g.Nodes[i].Renders > g.Nodes[j].Renders
		}
		return g.Nodes[i].Name < g.Nodes[j].Name
	})
	return g
}

// Longest-path layering from the roots of the kept subgraph; cycles (rare,
// from recursive components) are cut by the visit guard.
func assignDepth(nodes map[string]*GraphNode, keep map[string]bool, edges []GraphEdge) {
	children := map[string][]string{}
	hasParent := map[string]bool{}
	for _, e := range edges {
		children[e.From] = append(children[e.From], e.To)
		hasParent[e.To] = true
	}
	var visit func(name string, d int, path map[string]bool)
	visit = func(name string, d int, path map[string]bool) {
		n := nodes[name]
		if n == nil || path[name] {
			return
		}
		if d > n.Depth {
			n.Depth = d
		}
		path[name] = true
		for _, c := range children[name] {
			visit(c, d+1, path)
		}
		delete(path, name)
	}
	for name := range keep {
		if !hasParent[name] {
			visit(name, 0, map[string]bool{})
		}
	}
}
