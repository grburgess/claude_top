package bridge

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

// FallbackBackend is the last-resort backend for ttys owned by no known
// terminal. Only Interrupt works (SIGINT to the tty's foreground process
// group); everything else is ErrUnsupported.
type FallbackBackend struct {
	runPS func(ttyShort string) (string, error) // injectable ps for tests
	kill  func(pgid int) error                  // injectable kill for tests
}

var _ Backend = (*FallbackBackend)(nil)

// NewFallbackBackend returns the real ps/kill-backed implementation.
func NewFallbackBackend() *FallbackBackend { return &FallbackBackend{} }

func (f *FallbackBackend) ps(ttyShort string) (string, error) {
	if f.runPS != nil {
		return f.runPS(ttyShort)
	}
	out, err := exec.Command("ps", "-t", ttyShort, "-o", "pid,pgid,stat").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("ps -t %s: %w: %s", ttyShort, err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

// Enumerate discovers nothing.
func (f *FallbackBackend) Enumerate() ([]TermSession, error) { return nil, nil }

func (f *FallbackBackend) FocusTTY(tty string) error {
	return fmt.Errorf("%w: can't focus %s — terminal not recognized", ErrUnsupported, tty)
}

func (f *FallbackBackend) SendText(tty, text string, submit bool) error {
	return fmt.Errorf("%w: can't send text to %s — terminal not recognized", ErrUnsupported, tty)
}

func (f *FallbackBackend) NewTabAt(cwd, prefill string) error {
	return fmt.Errorf("%w: can't open a tab — no terminal backend available", ErrUnsupported)
}

func (f *FallbackBackend) ReopenAt(cwd, resumeCmd string) error {
	return f.NewTabAt(cwd, resumeCmd)
}

// Interrupt SIGINTs the foreground process group of tty.
func (f *FallbackBackend) Interrupt(tty string) error {
	out, err := f.ps(strings.TrimPrefix(tty, "/dev/"))
	if err != nil {
		return err
	}
	pgid, err := foregroundPgid(out)
	if err != nil {
		return fmt.Errorf("interrupt %s: %w", tty, err)
	}
	if f.kill != nil {
		return f.kill(pgid)
	}
	return syscall.Kill(-pgid, syscall.SIGINT)
}

// foregroundPgid picks the pgid of the foreground ('+' stat) process from
// `ps -o pid,pgid,stat` output.
func foregroundPgid(psOut string) (int, error) {
	for _, ln := range strings.Split(psOut, "\n") {
		fields := strings.Fields(ln)
		if len(fields) < 3 || !strings.Contains(fields[2], "+") {
			continue
		}
		pgid, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}
		return pgid, nil
	}
	return 0, fmt.Errorf("no foreground process group found")
}
