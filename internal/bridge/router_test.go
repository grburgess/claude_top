package bridge

import (
	"errors"
	"strings"
	"testing"
)

// testRouter wires three mocks into a Router.
func testRouter() (*Router, *MockITerm, *MockITerm, *MockITerm) {
	iterm := &MockITerm{Sessions: []TermSession{
		{TTY: "/dev/ttys001", BackendName: "iterm", WindowID: "1", TabIndex: 2},
	}}
	tmux := &MockITerm{Sessions: []TermSession{
		{TTY: "/dev/ttys050", BackendName: "tmux", WindowID: "main", TabIndex: 1},
	}}
	fallback := &MockITerm{}
	return NewRouter(iterm, tmux, fallback), iterm, tmux, fallback
}

func TestRouterRoutesByTTY(t *testing.T) {
	r, iterm, tmux, fallback := testRouter()
	if err := r.Interrupt("/dev/ttys050"); err != nil {
		t.Fatal(err)
	}
	if err := r.FocusTTY("/dev/ttys001"); err != nil {
		t.Fatal(err)
	}
	if err := r.SendText("/dev/ttys099", "hi", true); err != nil {
		t.Fatal(err)
	}
	if want := "Interrupt(/dev/ttys050)"; tmux.Calls[len(tmux.Calls)-1] != want {
		t.Errorf("tmux calls = %v, want last %s", tmux.Calls, want)
	}
	if want := "FocusTTY(/dev/ttys001)"; iterm.Calls[len(iterm.Calls)-1] != want {
		t.Errorf("iterm calls = %v, want last %s", iterm.Calls, want)
	}
	if want := `SendText(/dev/ttys099,"hi",true)`; len(fallback.Calls) != 1 || fallback.Calls[0] != want {
		t.Errorf("fallback calls = %v, want [%s]", fallback.Calls, want)
	}
}

func TestRouterTmuxWinsSharedTTY(t *testing.T) {
	r, iterm, tmux, _ := testRouter()
	iterm.Sessions = append(iterm.Sessions, TermSession{TTY: "/dev/ttys050", BackendName: "iterm"})
	if err := r.Interrupt("/dev/ttys050"); err != nil {
		t.Fatal(err)
	}
	for _, c := range iterm.Calls {
		if strings.HasPrefix(c, "Interrupt") {
			t.Errorf("shared tty routed to iterm: %v", iterm.Calls)
		}
	}
	if want := "Interrupt(/dev/ttys050)"; tmux.Calls[len(tmux.Calls)-1] != want {
		t.Errorf("tmux calls = %v, want last %s", tmux.Calls, want)
	}
}

func TestRouterResolveBackend(t *testing.T) {
	r, _, _, _ := testRouter()
	cases := map[string]string{
		"/dev/ttys050": "tmux",
		"/dev/ttys001": "iterm",
		"/dev/ttys099": "",
	}
	for tty, want := range cases {
		if got := r.ResolveBackend(tty); got != want {
			t.Errorf("ResolveBackend(%s) = %q, want %q", tty, got, want)
		}
	}
}

func TestRouterErrorsBubble(t *testing.T) {
	r, _, tmux, fallback := testRouter()
	// Prime the routing cache before poisoning the tmux mock: Err on a
	// MockITerm fails Enumerate too, which would demote the tty to fallback.
	r.ResolveBackend("/dev/ttys050")
	tmux.Err = errors.New("boom")
	if err := r.Interrupt("/dev/ttys050"); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Errorf("tmux error not bubbled: %v", err)
	}
	fallback.Err = ErrUnsupported
	if err := r.FocusTTY("/dev/ttys099"); !errors.Is(err, ErrUnsupported) {
		t.Errorf("fallback ErrUnsupported not bubbled: %v", err)
	}
}

func TestRouterEnumerateMerges(t *testing.T) {
	r, _, _, _ := testRouter()
	ss, err := r.Enumerate()
	if err != nil {
		t.Fatal(err)
	}
	if len(ss) != 2 {
		t.Fatalf("merged sessions = %d, want 2: %+v", len(ss), ss)
	}
}

func TestRouterCacheRespectsTTL(t *testing.T) {
	r, _, tmux, _ := testRouter()
	r.ResolveBackend("/dev/ttys050")
	before := len(tmux.Calls)
	r.ResolveBackend("/dev/ttys050") // within TTL: no re-enumerate
	if len(tmux.Calls) != before {
		t.Errorf("cache miss within TTL: calls = %v", tmux.Calls)
	}
}

func TestRouterNewTabPrefersITerm(t *testing.T) {
	r, iterm, tmux, _ := testRouter()
	if err := r.NewTabAt("/tmp", "claude"); err != nil {
		t.Fatal(err)
	}
	if want := `NewTabAt(/tmp,"claude")`; iterm.Calls[len(iterm.Calls)-1] != want {
		t.Errorf("iterm calls = %v, want last %s", iterm.Calls, want)
	}
	// No iTerm sessions: tmux takes creation verbs.
	iterm.Sessions = nil
	r2 := NewRouter(iterm, tmux, &MockITerm{})
	if err := r2.ReopenAt("/tmp", "claude -r x"); err != nil {
		t.Fatal(err)
	}
	if want := `ReopenAt(/tmp,"claude -r x")`; tmux.Calls[len(tmux.Calls)-1] != want {
		t.Errorf("tmux calls = %v, want last %s", tmux.Calls, want)
	}
}
