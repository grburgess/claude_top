// Package registry merges signal files, transcript stats and subagent
// counts into per-session views.
package registry

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/burgessj/claude_top/internal/signalfile"
	"github.com/burgessj/claude_top/internal/subagents"
	"github.com/burgessj/claude_top/internal/transcript"
)

// State is the liveness classification of a session.
type State int

const (
	StateDead State = iota
	StateRecent
	StateLive
)

func (s State) String() string {
	switch s {
	case StateLive:
		return "live"
	case StateRecent:
		return "recent"
	default:
		return "dead"
	}
}

// Mode selects which sessions List returns.
type Mode int

const (
	ModeLive Mode = iota
	ModeLiveRecent
	ModeAll
)

const (
	signalFreshWindow    = 30 * time.Minute
	liveTranscriptWindow = 10 * time.Minute
	noSignalLiveWindow   = 2 * time.Minute // signal-less machines: very fresh transcript = live
	recentWindow         = 30 * time.Minute
	subagentLiveWindow   = 120 * time.Second
)

// TermTab locates a terminal tab holding a session's tty.
type TermTab struct {
	WindowID string
	TabIndex int
	Backend  string // "iterm", "tmux", ... — which backend owns the tty
}

// Session is the merged view of one Claude Code session.
type Session struct {
	ID              string
	Signal          signalfile.Signal
	HasSignal       bool
	Stats           transcript.Stats
	TranscriptPath  string
	TranscriptMtime time.Time
	LiveAgents      int
	TotalAgents     int
	AgentNames      []string
	State           State
	TabIndex        int    // tab/window shortcut; valid when HasTab
	TabBackend      string // backend owning the tab ("iterm", "tmux")
	HasTab          bool

	// StatsReady reports that Stats/agent counts reflect the transcript
	// (set by EnsureStats; cleared when the transcript grows).
	StatsReady bool
	statsMtime time.Time // transcript mtime at last EnsureStats
}

// Registry scans the signal and projects directories on RefreshIndex.
// All exported methods are safe for concurrent use (guarded by mu), so
// EnsureStats may run from worker goroutines.
type Registry struct {
	SignalDir   string
	ProjectsDir string
	Now         func() time.Time // injectable clock

	mu       sync.Mutex
	readers  map[string]*transcript.Reader
	sessions map[string]*Session
	inflight map[string]bool    // EnsureStats parses in progress, by id
	termTabs map[string]TermTab // by tty
}

// SetTermTabs replaces the tty→tab map used to attach ⌘N shortcuts.
func (r *Registry) SetTermTabs(tabs map[string]TermTab) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.termTabs = tabs
}

// New builds a Registry. Empty dirs fall back to the standard locations.
func New(signalDir, projectsDir string) *Registry {
	if signalDir == "" {
		signalDir = signalfile.DefaultDir()
	}
	if projectsDir == "" {
		projectsDir = DefaultProjectsDir()
	}
	return &Registry{
		SignalDir:   signalDir,
		ProjectsDir: projectsDir,
		Now:         time.Now,
		readers:     map[string]*transcript.Reader{},
		sessions:    map[string]*Session{},
		inflight:    map[string]bool{},
	}
}

// DefaultProjectsDir returns the transcript root, honoring
// CLAUDE_TOP_PROJECTS_DIR.
func DefaultProjectsDir() string {
	if d := os.Getenv("CLAUDE_TOP_PROJECTS_DIR"); d != "" {
		return d
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".claude/projects"
	}
	return filepath.Join(home, ".claude", "projects")
}

// Refresh rescans both directories and synchronously parses every
// transcript (RefreshIndex + EnsureStats for all sessions). Kept for
// callers that want the old one-shot behavior; the TUI uses RefreshIndex
// plus background EnsureStats instead.
func (r *Registry) Refresh() error {
	if err := r.RefreshIndex(); err != nil {
		return err
	}
	r.mu.Lock()
	ids := make([]string, 0, len(r.sessions))
	for id := range r.sessions {
		ids = append(ids, id)
	}
	r.mu.Unlock()
	for _, id := range ids {
		r.EnsureStats(id)
	}
	return nil
}

// RefreshIndex is the cheap pass: it stats signal files and transcript
// mtimes only — no transcript parsing — and reclassifies session states.
func (r *Registry) RefreshIndex() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	seen := map[string]bool{}
	r.scanSignals(seen)
	r.scanTranscriptIndex(seen)
	now := r.Now()
	for id, s := range r.sessions {
		if !seen[id] {
			delete(r.sessions, id)
			delete(r.readers, id)
			continue
		}
		s.State = classify(s, now)
		if s.TranscriptPath == "" {
			s.StatsReady = true // nothing to parse
		} else if s.StatsReady && s.TranscriptMtime.After(s.statsMtime) {
			s.StatsReady = false // transcript grew: re-enrich
		}
	}
	return nil
}

// EnsureStats tails the session's transcript (and counts subagents) so its
// Stats are current, doing the expensive parse this session skipped during
// RefreshIndex. Reader offsets persist, so repeat calls are incremental.
// Returns false when the session is unknown, has no transcript, or a parse
// is already in flight.
func (r *Registry) EnsureStats(id string) bool {
	r.mu.Lock()
	s := r.sessions[id]
	if s == nil || s.TranscriptPath == "" || r.inflight[id] {
		r.mu.Unlock()
		return false
	}
	r.inflight[id] = true
	rd := r.readers[id]
	if rd == nil {
		rd = &transcript.Reader{}
		r.readers[id] = rd
	}
	rdCopy := *rd
	stats := s.Stats
	path := s.TranscriptPath
	mtime := s.TranscriptMtime
	r.mu.Unlock()

	err := rdCopy.Tail(path, &stats) // best effort
	live, total, names := subagents.Count(
		strings.TrimSuffix(path, ".jsonl"), subagentLiveWindow)

	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.inflight, id)
	s = r.sessions[id]
	if s == nil {
		return false
	}
	if err == nil {
		if cur, ok := r.readers[id]; ok {
			*cur = rdCopy
		}
		s.Stats = stats
	}
	s.LiveAgents, s.TotalAgents, s.AgentNames = live, total, names
	s.StatsReady = true
	s.statsMtime = mtime
	return true
}

func (r *Registry) scanSignals(seen map[string]bool) {
	entries, err := os.ReadDir(r.SignalDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		sig, err := signalfile.Parse(filepath.Join(r.SignalDir, e.Name()))
		if err != nil {
			continue
		}
		s := r.session(sig.SessionID)
		s.Signal = sig
		s.HasSignal = true
		seen[sig.SessionID] = true
	}
}

// scanTranscriptIndex records transcript paths and mtimes only; parsing is
// deferred to EnsureStats. Called with r.mu held.
func (r *Registry) scanTranscriptIndex(seen map[string]bool) {
	slugs, err := os.ReadDir(r.ProjectsDir)
	if err != nil {
		return
	}
	for _, slug := range slugs {
		if !slug.IsDir() {
			continue
		}
		slugDir := filepath.Join(r.ProjectsDir, slug.Name())
		files, err := os.ReadDir(slugDir)
		if err != nil {
			continue
		}
		for _, f := range files {
			if f.IsDir() || !strings.HasSuffix(f.Name(), ".jsonl") {
				continue
			}
			id := strings.TrimSuffix(f.Name(), ".jsonl")
			path := filepath.Join(slugDir, f.Name())
			s := r.session(id)
			s.TranscriptPath = path
			if info, err := f.Info(); err == nil {
				s.TranscriptMtime = info.ModTime()
			}
			seen[id] = true
		}
	}
}

func (r *Registry) session(id string) *Session {
	s := r.sessions[id]
	if s == nil {
		s = &Session{ID: id}
		r.sessions[id] = s
	}
	return s
}

func classify(s *Session, now time.Time) State {
	signalFresh := s.HasSignal && !s.Signal.Ts.IsZero() &&
		now.Sub(s.Signal.Ts) < signalFreshWindow
	transcriptFresh := !s.TranscriptMtime.IsZero() &&
		now.Sub(s.TranscriptMtime) < liveTranscriptWindow
	if (signalFresh && transcriptFresh) || (s.HasSignal && s.Signal.Type == "running") {
		return StateLive
	}
	// Signal-less machines (no iterm2-tab-status plugin): a very fresh
	// transcript alone is live — no attention/working granularity.
	if !s.HasSignal && !s.TranscriptMtime.IsZero() &&
		now.Sub(s.TranscriptMtime) < noSignalLiveWindow {
		return StateLive
	}
	if !s.TranscriptMtime.IsZero() && now.Sub(s.TranscriptMtime) < recentWindow {
		return StateRecent
	}
	return StateDead
}

// List returns sessions matching mode, ordered live-first then by most
// recent activity, then by ID. Sessions are copies, safe to read while
// background EnsureStats calls mutate the registry.
func (r *Registry) List(mode Mode) []*Session {
	r.mu.Lock()
	var out []*Session
	for _, s := range r.sessions {
		switch mode {
		case ModeLive:
			if s.State != StateLive {
				continue
			}
		case ModeLiveRecent:
			if s.State == StateDead {
				continue
			}
		}
		tab, ok := r.termTabs[s.Signal.TTY]
		s.TabIndex, s.HasTab = tab.TabIndex, ok && s.Signal.TTY != ""
		s.TabBackend = tab.Backend
		c := *s
		out = append(out, &c)
	}
	r.mu.Unlock()
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.State != b.State {
			return a.State > b.State
		}
		at, bt := a.activity(), b.activity()
		if !at.Equal(bt) {
			return at.After(bt)
		}
		return a.ID < b.ID
	})
	return out
}

func (s *Session) activity() time.Time {
	t := s.TranscriptMtime
	if s.HasSignal && s.Signal.Ts.After(t) {
		t = s.Signal.Ts
	}
	return t
}
