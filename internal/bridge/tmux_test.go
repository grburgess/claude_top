package bridge

import (
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const testSocket = "claude_top_test"

// tmuxForTest starts a scratch tmux server on a private socket with one
// detached session running `cat`, or skips when tmux is unusable.
func tmuxForTest(t *testing.T) *TmuxBackend {
	t.Helper()
	bin := tmuxBin()
	if _, err := exec.LookPath(bin); err != nil {
		t.Skipf("tmux not found (%s): %v", bin, err)
	}
	_ = exec.Command(bin, "-L", testSocket, "kill-server").Run() // stale server
	if out, err := exec.Command(bin, "-L", testSocket,
		"new-session", "-d", "-s", "t1", "cat").CombinedOutput(); err != nil {
		t.Skipf("cannot start scratch tmux server: %v: %s", err, out)
	}
	t.Cleanup(func() { _ = exec.Command(bin, "-L", testSocket, "kill-server").Run() })
	return &TmuxBackend{bin: bin, sockArgs: []string{"-L", testSocket}}
}

// waitFor polls cond every 100ms until it holds or d elapses.
func waitFor(d time.Duration, cond func() bool) bool {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return cond()
}

func TestTmuxEnumerateAndSendText(t *testing.T) {
	b := tmuxForTest(t)
	ss, err := b.Enumerate()
	if err != nil {
		t.Fatal(err)
	}
	if len(ss) == 0 {
		t.Fatal("Enumerate returned no panes")
	}
	p := ss[0]
	if p.BackendName != "tmux" || p.TTY == "" || !strings.HasPrefix(p.SessionID, "%") {
		t.Fatalf("bad pane: %+v", p)
	}
	if err := b.SendText(p.TTY, "hello from claude_top", true); err != nil {
		t.Fatal(err)
	}
	// cat echoes the submitted line back into the pane.
	if !waitFor(3*time.Second, func() bool {
		out, err := b.exec("capture-pane", "-p", "-t", p.SessionID)
		return err == nil && strings.Contains(out, "hello from claude_top")
	}) {
		out, _ := b.exec("capture-pane", "-p", "-t", p.SessionID)
		t.Errorf("sent text never arrived; pane:\n%s", out)
	}
}

func TestTmuxInterruptKillsSleep(t *testing.T) {
	b := tmuxForTest(t)
	out, err := b.exec("new-window", "-P", "-F", "#{pane_tty}", "sleep", "100")
	if err != nil {
		t.Fatal(err)
	}
	tty := strings.TrimSpace(out)
	if err := b.Interrupt(tty); err != nil {
		t.Fatal(err)
	}
	// The window's command dies on SIGINT, so its pane disappears.
	if !waitFor(3*time.Second, func() bool {
		ss, err := b.Enumerate()
		if err != nil {
			return false
		}
		for _, s := range ss {
			if s.TTY == tty {
				return false
			}
		}
		return true
	}) {
		t.Error("sleep pane survived Interrupt")
	}
}

func TestTmuxNewTabAtPrefillUnsubmitted(t *testing.T) {
	b := tmuxForTest(t)
	cwd := t.TempDir()
	const marker = "echo claude_top_marker"
	if err := b.NewTabAt(cwd, marker); err != nil {
		t.Fatal(err)
	}
	// Find the new pane: the one whose current path is cwd.
	var pane string
	if !waitFor(3*time.Second, func() bool {
		ss, err := b.Enumerate()
		if err != nil {
			return false
		}
		for _, s := range ss {
			out, err := b.exec("display-message", "-p", "-t", s.SessionID, "#{pane_current_path}")
			if err != nil {
				continue
			}
			got, _ := filepath.EvalSymlinks(strings.TrimSpace(out))
			want, _ := filepath.EvalSymlinks(cwd)
			if got == want && got != "" {
				pane = s.SessionID
				return true
			}
		}
		return false
	}) {
		t.Fatalf("no pane opened in %s", cwd)
	}
	if !waitFor(3*time.Second, func() bool {
		out, err := b.exec("capture-pane", "-p", "-t", pane)
		return err == nil && strings.Contains(out, marker)
	}) {
		out, _ := b.exec("capture-pane", "-p", "-t", pane)
		t.Fatalf("prefill missing from new pane:\n%s", out)
	}
	// Unsubmitted: the echo must not have produced its output line.
	out, _ := b.exec("capture-pane", "-p", "-t", pane)
	for _, ln := range strings.Split(out, "\n") {
		if strings.TrimSpace(ln) == "claude_top_marker" {
			t.Errorf("prefill was submitted; pane:\n%s", out)
		}
	}
}

func TestTmuxUnavailableTypedError(t *testing.T) {
	bin := tmuxBin()
	if _, err := exec.LookPath(bin); err == nil {
		// tmux present, no server on this socket.
		b := &TmuxBackend{bin: bin, sockArgs: []string{"-L", "claude_top_no_such_server"}}
		if _, err := b.Enumerate(); !errors.Is(err, ErrBackendUnavailable) {
			t.Errorf("no-server error = %v, want ErrBackendUnavailable", err)
		}
	}
	// Binary missing entirely.
	b := &TmuxBackend{bin: "/nonexistent/claude_top_tmux"}
	if _, err := b.Enumerate(); !errors.Is(err, ErrBackendUnavailable) {
		t.Errorf("missing-binary error = %v, want ErrBackendUnavailable", err)
	}
}
