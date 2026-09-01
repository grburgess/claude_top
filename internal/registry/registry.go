// Package registry merges signal files, transcript stats and subagent
// counts into per-session views.
package registry

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
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
	recentWindow         = 30 * time.Minute
	subagentLiveWindow   = 120 * time.Second
)

// TermTab locates a terminal tab holding a session's tty.
type TermTab struct {
	WindowID string
	TabIndex int
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
	TabIndex        int // ⌘N shortcut; valid when HasTab
	HasTab          bool
}

// Registry scans the signal and projects directories on Refresh.
type Registry struct {
	SignalDir   string
	ProjectsDir string
	Now         func() time.Time // injectable clock

	readers  map[string]*transcript.Reader
	sessions map[string]*Session
	termTabs map[string]TermTab // by tty
}

// SetTermTabs replaces the tty→tab map used to attach ⌘N shortcuts.
func (r *Registry) SetTermTabs(tabs map[string]TermTab) {
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

// Refresh rescans both directories and recomputes session states.
func (r *Registry) Refresh() error {
	seen := map[string]bool{}
	r.scanSignals(seen)
	r.scanTranscripts(seen)
	now := r.Now()
	for id, s := range r.sessions {
		if !seen[id] {
			delete(r.sessions, id)
			delete(r.readers, id)
			continue
		}
		s.State = classify(s, now)
	}
	return nil
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

func (r *Registry) scanTranscripts(seen map[string]bool) {
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
			rd := r.readers[id]
			if rd == nil {
				rd = &transcript.Reader{}
				r.readers[id] = rd
			}
			_ = rd.Tail(path, &s.Stats) // best effort
			s.LiveAgents, s.TotalAgents, s.AgentNames =
				subagents.Count(filepath.Join(slugDir, id), subagentLiveWindow)
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
	if !s.TranscriptMtime.IsZero() && now.Sub(s.TranscriptMtime) < recentWindow {
		return StateRecent
	}
	return StateDead
}

// List returns sessions matching mode, ordered live-first then by most
// recent activity, then by ID.
func (r *Registry) List(mode Mode) []*Session {
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
		out = append(out, s)
	}
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
