package bridge

import (
	"sync"
	"time"
)

// routerCacheTTL is how long Enumerate results stay fresh for tty routing.
const routerCacheTTL = 5 * time.Second

// Router implements Backend by delegating each call to the backend that
// owns the tty: tmux panes first, then iTerm sessions, else the fallback.
type Router struct {
	iterm, tmux, fallback Backend

	mu       sync.Mutex
	cachedAt time.Time
	owner    map[string]string // tty -> "tmux" | "iterm"
	sessions []TermSession     // merged tmux + iterm sessions
	now      func() time.Time  // injectable for tests
}

var _ Backend = (*Router)(nil)

// NewRouter builds a Router over the three backends.
func NewRouter(iterm, tmux, fallback Backend) *Router {
	return &Router{iterm: iterm, tmux: tmux, fallback: fallback, now: time.Now}
}

// refreshLocked re-enumerates tmux and iTerm (errors treated as "no
// sessions") and rebuilds the tty ownership map. Callers hold r.mu.
func (r *Router) refreshLocked() {
	owner := map[string]string{}
	var merged []TermSession
	tmuxSessions, _ := r.tmux.Enumerate()
	for _, s := range tmuxSessions {
		owner[s.TTY] = "tmux"
	}
	merged = append(merged, tmuxSessions...)
	itermSessions, _ := r.iterm.Enumerate()
	for _, s := range itermSessions {
		if _, taken := owner[s.TTY]; !taken {
			owner[s.TTY] = "iterm"
		}
	}
	merged = append(merged, itermSessions...)
	r.owner, r.sessions, r.cachedAt = owner, merged, r.now()
}

// ensureFreshLocked refreshes the cache when it is older than the TTL.
func (r *Router) ensureFreshLocked() {
	if r.owner == nil || r.now().Sub(r.cachedAt) > routerCacheTTL {
		r.refreshLocked()
	}
}

// backendFor resolves which backend owns tty; unknown ttys get the fallback.
func (r *Router) backendFor(tty string) (Backend, string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ensureFreshLocked()
	switch r.owner[tty] {
	case "tmux":
		return r.tmux, "tmux"
	case "iterm":
		return r.iterm, "iterm"
	}
	return r.fallback, ""
}

// ResolveBackend names the backend owning tty ("tmux", "iterm") or "" when
// unknown (fallback). For UI display.
func (r *Router) ResolveBackend(tty string) string {
	_, name := r.backendFor(tty)
	return name
}

// creationBackend picks where tab-creating verbs go: iTerm when it has any
// sessions, else tmux when it has any, else the fallback.
func (r *Router) creationBackend() Backend {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ensureFreshLocked()
	hasTmux, hasITerm := false, false
	for _, s := range r.sessions {
		switch s.BackendName {
		case "tmux":
			hasTmux = true
		case "iterm":
			hasITerm = true
		}
	}
	switch {
	case hasITerm:
		return r.iterm
	case hasTmux:
		return r.tmux
	}
	return r.fallback
}

// Enumerate force-refreshes and returns the merged tmux + iTerm sessions.
func (r *Router) Enumerate() ([]TermSession, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.refreshLocked()
	return r.sessions, nil
}

func (r *Router) FocusTTY(tty string) error {
	b, _ := r.backendFor(tty)
	return b.FocusTTY(tty)
}

func (r *Router) SendText(tty, text string, submit bool) error {
	b, _ := r.backendFor(tty)
	return b.SendText(tty, text, submit)
}

func (r *Router) Interrupt(tty string) error {
	b, _ := r.backendFor(tty)
	return b.Interrupt(tty)
}

func (r *Router) NewTabAt(cwd, prefill string) error {
	return r.creationBackend().NewTabAt(cwd, prefill)
}

func (r *Router) ReopenAt(cwd, resumeCmd string) error {
	return r.creationBackend().ReopenAt(cwd, resumeCmd)
}
