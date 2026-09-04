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

// signalPid is the pid every writeSignal fixture records; openPids decides
// whether the fake liveness probe reports it alive (terminal still open).
const signalPid = 1

type fixture struct {
	r           *Registry
	signalDir   string
	projectsDir string
	slugDir     string
	openPids    map[int]bool
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
	f := &fixture{
		r: r, signalDir: signalDir, projectsDir: projectsDir, slugDir: slugDir,
		openPids: map[int]bool{},
	}
	r.Alive = func(pid int) bool { return f.openPids[pid] }
	return f
}

// openTerminal makes the fixture's signal pid probe as alive, i.e. the
// session's tab is still open.
func (f *fixture) openTerminal() { f.openPids[signalPid] = true }

func (f *fixture) writeSignal(t *testing.T, id, typ string, ts time.Time) {
	t.Helper()
	f.writeSignalPid(t, id, typ, ts, signalPid)
}

func (f *fixture) writeSignalPid(t *testing.T, id, typ string, ts time.Time, pid int) {
	t.Helper()
	content := fmt.Sprintf(
		`{"session_id":"%s","type":"%s","message":"m","project":"p","cwd":"/Users/x/proj","tty":"/dev/ttys001","pid":"%d","ts":"%d"}`,
		id, typ, pid, ts.Unix())
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

func (f *fixture) sessionOf(t *testing.T, id string) *Session {
	t.Helper()
	if err := f.r.Refresh(); err != nil {
		t.Fatal(err)
	}
	for _, s := range f.r.List(ModeAll) {
		if s.ID == id {
			return s
		}
	}
	t.Fatalf("session %s not found", id)
	return nil
}

func (f *fixture) stateOf(t *testing.T, id string) State {
	t.Helper()
	return f.sessionOf(t, id).State
}

func TestStateLiveFreshSignalAndTranscript(t *testing.T) {
	f := newFixture(t)
	f.openTerminal()
	f.writeSignal(t, "s1", "idle", now.Add(-5*time.Minute))
	f.writeTranscript(t, "s1", now.Add(-2*time.Minute))
	if got := f.stateOf(t, "s1"); got != StateLive {
		t.Errorf("state = %v, want live", got)
	}
}

// TestStateLiveOpenTerminalIdleForHours is the definition of live: the tab is
// open, so the session is live no matter how long ago it last did anything.
func TestStateLiveOpenTerminalIdleForHours(t *testing.T) {
	f := newFixture(t)
	f.openTerminal()
	f.writeSignal(t, "s2", "idle", now.Add(-2*time.Hour))
	f.writeTranscript(t, "s2", now.Add(-2*time.Hour))
	s := f.sessionOf(t, "s2")
	if !s.Open {
		t.Error("Open = false for a session whose pid is alive")
	}
	if s.State != StateLive {
		t.Errorf("state = %v, want live (terminal open)", s.State)
	}
}

// TestStateNotLiveWhenTerminalClosed is the mirror: a signal fresh enough to
// pass every time window still is not live once its pid is gone.
func TestStateNotLiveWhenTerminalClosed(t *testing.T) {
	f := newFixture(t) // pid never marked open
	f.writeSignal(t, "s2b", "running", now.Add(-1*time.Minute))
	f.writeTranscript(t, "s2b", now.Add(-1*time.Minute))
	s := f.sessionOf(t, "s2b")
	if s.Open {
		t.Error("Open = true for a session whose pid is gone")
	}
	if s.State == StateLive {
		t.Error("state = live for a closed terminal")
	}
}

// TestStateLiveRunningSignalWithoutPid keeps the timestamp fallback working
// for signals that carry no pid (older plugin): openness is undecidable
// there, so `type: running` still means live.
func TestStateLiveRunningSignalWithoutPid(t *testing.T) {
	f := newFixture(t)
	f.writeSignalPid(t, "s2c", "running", now.Add(-2*time.Hour), 0)
	f.writeTranscript(t, "s2c", now.Add(-2*time.Hour))
	if got := f.stateOf(t, "s2c"); got != StateLive {
		t.Errorf("state = %v, want live (type running, no pid to probe)", got)
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

func TestStateLiveNoSignalFreshTranscript(t *testing.T) {
	f := newFixture(t)
	// no signal file at all, transcript 1 min old -> live (signal-less machine)
	f.writeTranscript(t, "ns1", now.Add(-1*time.Minute))
	if got := f.stateOf(t, "ns1"); got != StateLive {
		t.Errorf("state = %v, want live (no signal, fresh transcript)", got)
	}
}

func TestStateRecentNoSignalStaleTranscript(t *testing.T) {
	f := newFixture(t)
	// no signal file, transcript 5 min old (not <2min) -> recent
	f.writeTranscript(t, "ns2", now.Add(-5*time.Minute))
	if got := f.stateOf(t, "ns2"); got != StateRecent {
		t.Errorf("state = %v, want recent (no signal, 5m-old transcript)", got)
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

// TestStateTransitionOnCloseThenClock: an open session stays live as the
// clock runs; closing the terminal drops it to recent, and only then does
// age carry it to dead.
func TestStateTransitionOnCloseThenClock(t *testing.T) {
	f := newFixture(t)
	f.openTerminal()
	f.writeSignal(t, "s6", "idle", now.Add(-5*time.Minute))
	f.writeTranscript(t, "s6", now.Add(-5*time.Minute))
	if got := f.stateOf(t, "s6"); got != StateLive {
		t.Fatalf("state = %v, want live", got)
	}
	saved := now
	defer func() { now = saved }()

	// clock alone must not demote an open session
	now = saved.Add(20 * time.Minute)
	if got := f.stateOf(t, "s6"); got != StateLive {
		t.Errorf("state after +20m (still open) = %v, want live", got)
	}
	// close the terminal: transcript is 25 min old -> recent
	delete(f.openPids, signalPid)
	if got := f.stateOf(t, "s6"); got != StateRecent {
		t.Errorf("state after close = %v, want recent", got)
	}
	// and 40 min old -> dead
	now = saved.Add(40 * time.Minute)
	if got := f.stateOf(t, "s6"); got != StateDead {
		t.Errorf("state after +40m = %v, want dead", got)
	}
}

// writeTitledTranscript writes a transcript carrying an aiTitle, so the
// session has the title its terminal tab is matched against.
func (f *fixture) writeTitledTranscript(t *testing.T, id, title string, mtime time.Time) {
	t.Helper()
	p := filepath.Join(f.slugDir, id+".jsonl")
	content := `{"type":"assistant","message":{"model":"claude-opus-4","usage":{"input_tokens":1,"output_tokens":2}},"timestamp":"2026-01-01T11:00:00Z"}` + "\n" +
		fmt.Sprintf(`{"type":"ai-title","aiTitle":%q,"timestamp":"2026-01-01T11:00:00Z"}`, title) + "\n"
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(p, mtime, mtime); err != nil {
		t.Fatal(err)
	}
}

// TestOpenByTabTitle is the case the pid probe cannot see: no signal file at
// all (the plugin's signal files are short-lived), but a terminal tab still
// carries the session's title, which proves the terminal is open and names
// the tty to jump to.
func TestOpenByTabTitle(t *testing.T) {
	f := newFixture(t)
	f.writeTitledTranscript(t, "tt1", "Complaints in Scope MDD draft", now.Add(-2*time.Hour))
	f.r.SetTermTabs(map[string]TermTab{
		"/dev/ttys027": {TabIndex: 8, Backend: "iterm",
			Title: "◑ Complaints in Scope MDD draft (toolbox) (toolbox)"},
	})
	s := f.sessionOf(t, "tt1")
	if !s.Open {
		t.Error("Open = false although a tab carries the session title")
	}
	if s.TTY != "/dev/ttys027" {
		t.Errorf("TTY = %q, want /dev/ttys027", s.TTY)
	}
	if s.State != StateLive {
		t.Errorf("state = %v, want live", s.State)
	}
	if !s.HasTab || s.TabIndex != 8 {
		t.Errorf("HasTab/TabIndex = %v/%d, want true/8", s.HasTab, s.TabIndex)
	}
}

// TestNoOpenWhenTabTitleReverted covers the session ending: the shell takes
// its tab title back, so nothing matches and the session is closed.
func TestNoOpenWhenTabTitleReverted(t *testing.T) {
	f := newFixture(t)
	f.writeTitledTranscript(t, "tt2", "Complaints in Scope MDD draft", now.Add(-2*time.Hour))
	f.r.SetTermTabs(map[string]TermTab{
		"/dev/ttys027": {TabIndex: 8, Title: "burgessj: projects (-zsh)"},
	})
	s := f.sessionOf(t, "tt2")
	if s.Open || s.TTY != "" {
		t.Errorf("Open/TTY = %v/%q, want false/\"\"", s.Open, s.TTY)
	}
	if s.State == StateLive {
		t.Error("state = live for a session no tab is showing")
	}
}

// TestShortTitleDoesNotClaimTab: a stub title must not match half the
// terminal, so titles under minTitleMatch never claim a tab.
func TestShortTitleDoesNotClaimTab(t *testing.T) {
	f := newFixture(t)
	f.writeTitledTranscript(t, "tt3", "Fix", now.Add(-2*time.Hour))
	f.r.SetTermTabs(map[string]TermTab{"/dev/ttys009": {Title: "Fix the reducer bugs"}})
	if s := f.sessionOf(t, "tt3"); s.Open {
		t.Error("short title claimed a tab")
	}
}

// TestSignalTTYWinsOverTitleMatch: the signal names the tty authoritatively,
// so a title match must not move where actions are sent.
func TestSignalTTYWinsOverTitleMatch(t *testing.T) {
	f := newFixture(t)
	f.openTerminal()
	f.writeSignal(t, "tt4", "idle", now.Add(-5*time.Minute))
	f.writeTitledTranscript(t, "tt4", "Complaints in Scope MDD draft", now.Add(-5*time.Minute))
	f.r.SetTermTabs(map[string]TermTab{
		"/dev/ttys027": {TabIndex: 8, Title: "◑ Complaints in Scope MDD draft"},
	})
	if s := f.sessionOf(t, "tt4"); s.TTY != "/dev/ttys001" {
		t.Errorf("TTY = %q, want the signal's /dev/ttys001", s.TTY)
	}
}

func TestListModesAndOrder(t *testing.T) {
	f := newFixture(t)
	f.openTerminal()
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

func TestSetTermTabsAttachesShortcut(t *testing.T) {
	f := newFixture(t)
	f.writeSignal(t, "s9", "running", now.Add(-1*time.Minute))
	f.r.SetTermTabs(map[string]TermTab{
		"/dev/ttys001": {WindowID: "w1", TabIndex: 6},
	})
	if err := f.r.Refresh(); err != nil {
		t.Fatal(err)
	}
	ss := f.r.List(ModeAll)
	if len(ss) != 1 {
		t.Fatalf("sessions = %d, want 1", len(ss))
	}
	if !ss[0].HasTab || ss[0].TabIndex != 6 {
		t.Errorf("HasTab/TabIndex = %v/%d, want true/6", ss[0].HasTab, ss[0].TabIndex)
	}
	f.r.SetTermTabs(map[string]TermTab{"/dev/ttys099": {TabIndex: 2}})
	if ss := f.r.List(ModeAll); ss[0].HasTab {
		t.Error("HasTab = true for unmatched tty")
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
