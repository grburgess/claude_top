package registry

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// fixed "now" for the fake clock
var now = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

type fixture struct {
	r           *Registry
	signalDir   string
	projectsDir string
	slugDir     string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	signalDir := t.TempDir()
	projectsDir := t.TempDir()
	slugDir := filepath.Join(projectsDir, "-Users-x-proj")
	if err := os.MkdirAll(slugDir, 0o755); err != nil {
		t.Fatal(err)
	}
	r := New(signalDir, projectsDir)
	r.Now = func() time.Time { return now }
	return &fixture{r: r, signalDir: signalDir, projectsDir: projectsDir, slugDir: slugDir}
}

func (f *fixture) writeSignal(t *testing.T, id, typ string, ts time.Time) {
	t.Helper()
	content := fmt.Sprintf(
		`{"session_id":"%s","type":"%s","message":"m","project":"p","cwd":"/Users/x/proj","tty":"/dev/ttys001","pid":"1","ts":"%d"}`,
		id, typ, ts.Unix())
	if err := os.WriteFile(filepath.Join(f.signalDir, id+".json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (f *fixture) writeTranscript(t *testing.T, id string, mtime time.Time) {
	t.Helper()
	p := filepath.Join(f.slugDir, id+".jsonl")
	content := `{"type":"assistant","message":{"model":"claude-opus-4","usage":{"input_tokens":1,"output_tokens":2}},"timestamp":"2026-01-01T11:00:00Z"}` + "\n"
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(p, mtime, mtime); err != nil {
		t.Fatal(err)
	}
}

func (f *fixture) stateOf(t *testing.T, id string) State {
	t.Helper()
	if err := f.r.Refresh(); err != nil {
		t.Fatal(err)
	}
	for _, s := range f.r.List(ModeAll) {
		if s.ID == id {
			return s.State
		}
	}
	t.Fatalf("session %s not found", id)
	return StateDead
}

func TestStateLiveFreshSignalAndTranscript(t *testing.T) {
	f := newFixture(t)
	f.writeSignal(t, "s1", "idle", now.Add(-5*time.Minute))
	f.writeTranscript(t, "s1", now.Add(-2*time.Minute))
	if got := f.stateOf(t, "s1"); got != StateLive {
		t.Errorf("state = %v, want live", got)
	}
}

func TestStateLiveRunningSignalStaleTranscript(t *testing.T) {
	f := newFixture(t)
	f.writeSignal(t, "s2", "running", now.Add(-2*time.Hour))
	f.writeTranscript(t, "s2", now.Add(-2*time.Hour))
	if got := f.stateOf(t, "s2"); got != StateLive {
		t.Errorf("state = %v, want live (type running)", got)
	}
}

func TestStateRecent(t *testing.T) {
	f := newFixture(t)
	// signal stale, transcript 20 min old -> recent
	f.writeSignal(t, "s3", "idle", now.Add(-1*time.Hour))
	f.writeTranscript(t, "s3", now.Add(-20*time.Minute))
	if got := f.stateOf(t, "s3"); got != StateRecent {
		t.Errorf("state = %v, want recent", got)
	}
}

func TestStateRecentTranscriptOnly(t *testing.T) {
	f := newFixture(t)
	// fresh signal but transcript 15 min old (not <10min) -> recent
	f.writeSignal(t, "s4", "idle", now.Add(-5*time.Minute))
	f.writeTranscript(t, "s4", now.Add(-15*time.Minute))
	if got := f.stateOf(t, "s4"); got != StateRecent {
		t.Errorf("state = %v, want recent", got)
	}
}

func TestStateDead(t *testing.T) {
	f := newFixture(t)
	f.writeSignal(t, "s5", "idle", now.Add(-2*time.Hour))
	f.writeTranscript(t, "s5", now.Add(-2*time.Hour))
	if got := f.stateOf(t, "s5"); got != StateDead {
		t.Errorf("state = %v, want dead", got)
	}
}

func TestStateTransitionByClock(t *testing.T) {
	f := newFixture(t)
	f.writeSignal(t, "s6", "idle", now.Add(-5*time.Minute))
	f.writeTranscript(t, "s6", now.Add(-5*time.Minute))
	if got := f.stateOf(t, "s6"); got != StateLive {
		t.Fatalf("state = %v, want live", got)
	}
	// advance fake clock 20 min: transcript now 25 min old -> recent
	saved := now
	defer func() { now = saved }()
	now = now.Add(20 * time.Minute)
	if got := f.stateOf(t, "s6"); got != StateRecent {
		t.Errorf("state after +20m = %v, want recent", got)
	}
	// advance to 40 min old -> dead
	now = saved.Add(40 * time.Minute)
	if got := f.stateOf(t, "s6"); got != StateDead {
		t.Errorf("state after +40m = %v, want dead", got)
	}
}

func TestListModesAndOrder(t *testing.T) {
	f := newFixture(t)
	f.writeSignal(t, "live1", "running", now.Add(-1*time.Minute))
	f.writeTranscript(t, "live1", now.Add(-1*time.Minute))
	f.writeTranscript(t, "recent1", now.Add(-20*time.Minute))
	f.writeTranscript(t, "dead1", now.Add(-2*time.Hour))
	if err := f.r.Refresh(); err != nil {
		t.Fatal(err)
	}
	if got := len(f.r.List(ModeLive)); got != 1 {
		t.Errorf("ModeLive count = %d, want 1", got)
	}
	if got := len(f.r.List(ModeLiveRecent)); got != 2 {
		t.Errorf("ModeLiveRecent count = %d, want 2", got)
	}
	all := f.r.List(ModeAll)
	if len(all) != 3 {
		t.Fatalf("ModeAll count = %d, want 3", len(all))
	}
	if all[0].ID != "live1" || all[1].ID != "recent1" || all[2].ID != "dead1" {
		t.Errorf("order = %s,%s,%s", all[0].ID, all[1].ID, all[2].ID)
	}
}

func TestRefreshMergesStatsAndKeepsOffsets(t *testing.T) {
	f := newFixture(t)
	f.writeTranscript(t, "s7", now.Add(-1*time.Minute))
	if err := f.r.Refresh(); err != nil {
		t.Fatal(err)
	}
	s := f.r.List(ModeAll)[0]
	if s.Stats.Turns != 1 || s.Stats.Model != "claude-opus-4" {
		t.Fatalf("stats not merged: %+v", s.Stats)
	}
	// second refresh must not double-count (reader offset preserved)
	if err := f.r.Refresh(); err != nil {
		t.Fatal(err)
	}
	s = f.r.List(ModeAll)[0]
	if s.Stats.Turns != 1 {
		t.Errorf("Turns = %d after second refresh, want 1", s.Stats.Turns)
	}
}

func TestRemovedSessionDropped(t *testing.T) {
	f := newFixture(t)
	f.writeTranscript(t, "s8", now.Add(-1*time.Minute))
	if err := f.r.Refresh(); err != nil {
		t.Fatal(err)
	}
	if len(f.r.List(ModeAll)) != 1 {
		t.Fatal("expected 1 session")
	}
	if err := os.Remove(filepath.Join(f.slugDir, "s8.jsonl")); err != nil {
		t.Fatal(err)
	}
	if err := f.r.Refresh(); err != nil {
		t.Fatal(err)
	}
	if got := len(f.r.List(ModeAll)); got != 0 {
		t.Errorf("sessions after removal = %d, want 0", got)
	}
}
