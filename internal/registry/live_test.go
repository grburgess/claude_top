package registry

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/burgessj/claude_top/internal/signalfile"
)

// liveDeadline bounds how long a newly written signal/transcript may take to
// show up in a Refresh+List.
const liveDeadline = 2 * time.Second

type liveFixture struct {
	r           *Registry
	signalDir   string
	projectsDir string
	slugDir     string
}

// newLiveFixture builds a Registry on real (empty) temp dirs with the real
// clock, so states are computed against wall time like production.
func newLiveFixture(t *testing.T) *liveFixture {
	t.Helper()
	signalDir := t.TempDir()
	projectsDir := t.TempDir()
	slugDir := filepath.Join(projectsDir, "-Users-x-live")
	if err := os.MkdirAll(slugDir, 0o755); err != nil {
		t.Fatal(err)
	}
	return &liveFixture{
		r:           New(signalDir, projectsDir),
		signalDir:   signalDir,
		projectsDir: projectsDir,
		slugDir:     slugDir,
	}
}

func (f *liveFixture) find(t *testing.T, mode Mode, id string) *Session {
	t.Helper()
	if err := f.r.Refresh(); err != nil {
		t.Fatal(err)
	}
	for _, s := range f.r.List(mode) {
		if s.ID == id {
			return s
		}
	}
	return nil
}

// TestLiveSignalAppearsWithinDeadline drives the real fsnotify watcher: an
// empty registry, a first Refresh, then a synthetic signal file written after
// it. The session must be visible as live within liveDeadline.
func TestLiveSignalAppearsWithinDeadline(t *testing.T) {
	f := newLiveFixture(t)

	if err := f.r.Refresh(); err != nil {
		t.Fatal(err)
	}
	if got := len(f.r.List(ModeAll)); got != 0 {
		t.Fatalf("sessions before write = %d, want 0", got)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events, err := signalfile.WatchDir(ctx, f.signalDir)
	if err != nil {
		t.Fatal(err)
	}

	const id = "live-sig-1"
	ts := time.Now()
	// the test process stands in for the tab's login process: a real pid the
	// production liveness probe reports alive, so the session reads as open
	pid := os.Getpid()
	content := fmt.Sprintf(
		`{"session_id":"%s","type":"running","message":"m","project":"liveproj","cwd":"/Users/x/live","tty":"/dev/ttys042","pid":"%d","ts":"%d"}`,
		id, pid, ts.Unix())

	start := time.Now()
	if err := os.WriteFile(filepath.Join(f.signalDir, id+".json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	select {
	case sig := <-events:
		if sig.SessionID != id {
			t.Fatalf("watcher session_id = %q, want %q", sig.SessionID, id)
		}
	case <-time.After(liveDeadline):
		t.Fatalf("no watcher event within %v", liveDeadline)
	}

	s := f.find(t, ModeLive, id)
	elapsed := time.Since(start)
	if s == nil {
		t.Fatalf("session %s absent from ModeLive after watcher event (%v)", id, elapsed)
	}
	if elapsed > liveDeadline {
		t.Errorf("latency = %v, want <= %v", elapsed, liveDeadline)
	}
	if s.State != StateLive {
		t.Errorf("state = %v, want live", s.State)
	}
	if !s.HasSignal {
		t.Error("HasSignal = false")
	}
	if s.Signal.Project != "liveproj" {
		t.Errorf("project = %q, want %q", s.Signal.Project, "liveproj")
	}
	if s.Signal.Cwd != "/Users/x/live" || s.Signal.Pid != pid {
		t.Errorf("cwd/pid = %q/%d, want /Users/x/live/%d", s.Signal.Cwd, s.Signal.Pid, pid)
	}
	if !s.Open {
		t.Error("Open = false although the signal pid is alive")
	}
	t.Logf("C1 signal latency: %v (deadline %v)", elapsed, liveDeadline)
}

// TestLiveTranscriptOnlySessionAppears covers a session with no signal file:
// a .jsonl created after the first Refresh must show up in ModeAll, polled
// until liveDeadline.
func TestLiveTranscriptOnlySessionAppears(t *testing.T) {
	f := newLiveFixture(t)

	if err := f.r.Refresh(); err != nil {
		t.Fatal(err)
	}
	if got := len(f.r.List(ModeAll)); got != 0 {
		t.Fatalf("sessions before write = %d, want 0", got)
	}

	const id = "live-tx-1"
	line := `{"type":"assistant","message":{"model":"claude-opus-4","usage":{"input_tokens":1,"output_tokens":2}},"timestamp":"2026-01-01T11:00:00Z"}` + "\n"

	start := time.Now()
	if err := os.WriteFile(filepath.Join(f.slugDir, id+".jsonl"), []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}

	var s *Session
	for time.Since(start) < liveDeadline {
		if s = f.find(t, ModeAll, id); s != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	elapsed := time.Since(start)
	if s == nil {
		t.Fatalf("transcript-only session %s absent from ModeAll within %v", id, liveDeadline)
	}
	if s.HasSignal {
		t.Error("HasSignal = true for transcript-only session")
	}
	// no signal file (machine without the iterm2-tab-status plugin): a
	// just-written transcript alone classifies as live
	if s.State != StateLive {
		t.Errorf("state = %v, want live (fresh transcript, no signal)", s.State)
	}
	if s.Stats.Turns != 1 {
		t.Errorf("Turns = %d, want 1", s.Stats.Turns)
	}
	t.Logf("C1 transcript-only latency: %v (deadline %v)", elapsed, liveDeadline)
}
