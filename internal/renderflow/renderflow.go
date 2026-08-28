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
