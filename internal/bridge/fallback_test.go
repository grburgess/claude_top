package bridge

import (
	"errors"
	"testing"
)

const fakePS = `  PID  PGID STAT
  501   501 Ss
 1234  1234 S+
 1235  1234 S+
`

func TestForegroundPgid(t *testing.T) {
	pgid, err := foregroundPgid(fakePS)
	if err != nil {
		t.Fatal(err)
	}
	if pgid != 1234 {
		t.Errorf("pgid = %d, want 1234", pgid)
	}
	if _, err := foregroundPgid("  PID  PGID STAT\n  501   501 Ss\n"); err == nil {
		t.Error("no '+' row must error")
	}
}

func TestFallbackInterruptKillsForegroundGroup(t *testing.T) {
	var gotTTY string
	var gotPgid int
	f := &FallbackBackend{
		runPS: func(ttyShort string) (string, error) { gotTTY = ttyShort; return fakePS, nil },
		kill:  func(pgid int) error { gotPgid = pgid; return nil },
	}
	if err := f.Interrupt("/dev/ttys004"); err != nil {
		t.Fatal(err)
	}
	if gotTTY != "ttys004" {
		t.Errorf("ps tty = %q, want ttys004 (short form)", gotTTY)
	}
	if gotPgid != 1234 {
		t.Errorf("killed pgid = %d, want 1234", gotPgid)
	}
}

func TestFallbackUnsupportedVerbs(t *testing.T) {
	f := NewFallbackBackend()
	for name, err := range map[string]error{
		"FocusTTY": f.FocusTTY("/dev/ttys004"),
		"SendText": f.SendText("/dev/ttys004", "hi", true),
		"NewTabAt": f.NewTabAt("/tmp", "claude"),
		"ReopenAt": f.ReopenAt("/tmp", "claude -r x"),
	} {
		if !errors.Is(err, ErrUnsupported) {
			t.Errorf("%s = %v, want ErrUnsupported", name, err)
		}
	}
	ss, err := f.Enumerate()
	if err != nil || len(ss) != 0 {
		t.Errorf("Enumerate = %v, %v, want empty, nil", ss, err)
	}
}
