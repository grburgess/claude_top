package bridge

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// TmuxBackend implements Backend by exec-ing the tmux binary.
// bin/sockArgs/run are injectable for tests; zero values mean the resolved
// tmux binary on the default server socket.
type TmuxBackend struct {
	bin      string   // tmux binary; "" resolves via tmuxBin
	sockArgs []string // prepended args, e.g. {"-L", "socket"} for tests
	run      func(args ...string) (string, error)
}

var _ Backend = (*TmuxBackend)(nil)

// NewTmuxBackend returns the real tmux-backed implementation.
func NewTmuxBackend() *TmuxBackend { return &TmuxBackend{} }

// tmuxBin resolves the tmux binary: $TMUX_BIN, then PATH, then the
// homebrew default.
func tmuxBin() string {
	if b := os.Getenv("TMUX_BIN"); b != "" {
		return b
	}
	if p, err := exec.LookPath("tmux"); err == nil {
		return p
	}
	return "/opt/homebrew/bin/tmux"
}

func (t *TmuxBackend) exec(args ...string) (string, error) {
	if t.run != nil {
		return t.run(args...)
	}
	bin := t.bin
	if bin == "" {
		bin = tmuxBin()
	}
	full := append(append([]string{}, t.sockArgs...), args...)
	out, err := exec.Command(bin, full...).CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if tmuxUnavailable(err, msg) {
			return "", fmt.Errorf("%w: tmux: %v: %s", ErrBackendUnavailable, err, msg)
		}
		return "", fmt.Errorf("tmux %s: %w: %s", args[0], err, msg)
	}
	return string(out), nil
}

// tmuxUnavailable classifies "tmux missing" and "no server" failures.
func tmuxUnavailable(err error, out string) bool {
	if errors.Is(err, exec.ErrNotFound) || errors.Is(err, os.ErrNotExist) {
		return true
	}
	return strings.Contains(out, "no server running") ||
		strings.Contains(out, "error connecting to")
}

const tmuxPaneFormat = "#{pane_tty}\t#{session_name}\t#{window_index}\t#{pane_id}\t#{window_name}"

// Enumerate lists all panes on the tmux server.
func (t *TmuxBackend) Enumerate() ([]TermSession, error) {
	out, err := t.exec("list-panes", "-a", "-F", tmuxPaneFormat)
	if err != nil {
		return nil, err
	}
	return parseTmuxPanes(out), nil
}

func parseTmuxPanes(out string) []TermSession {
	var sessions []TermSession
	for _, ln := range strings.Split(out, "\n") {
		ln = strings.TrimRight(ln, "\r")
		if ln == "" {
			continue
		}
		parts := strings.SplitN(ln, "\t", 5)
		if len(parts) != 5 {
			continue
		}
		idx, _ := strconv.Atoi(parts[2])
		sessions = append(sessions, TermSession{
			TTY:         parts[0],
			WindowID:    parts[1], // tmux session name
			TabIndex:    idx,
			SessionID:   parts[3], // tmux pane id (%N)
			Title:       parts[4],
			BackendName: "tmux",
		})
	}
	return sessions
}

// paneFor finds the pane owning tty.
func (t *TmuxBackend) paneFor(tty string) (TermSession, error) {
	sessions, err := t.Enumerate()
	if err != nil {
		return TermSession{}, err
	}
	for _, s := range sessions {
		if s.TTY == tty {
			return s, nil
		}
	}
	return TermSession{}, fmt.Errorf("tmux: no pane owns tty %s", tty)
}

// FocusTTY selects the window holding tty and switches the attached client
// to its session (best effort when no client is attached).
func (t *TmuxBackend) FocusTTY(tty string) error {
	p, err := t.paneFor(tty)
	if err != nil {
		return err
	}
	if _, err := t.exec("select-window", "-t", p.SessionID); err != nil {
		return err
	}
	_, _ = t.exec("switch-client", "-t", p.WindowID) // best effort
	return nil
}

// SendText types text literally into the pane owning tty; submit appends
// Enter. Text is passed as a single arg after -l -- so `;` and leading
// dashes are never interpreted by tmux.
func (t *TmuxBackend) SendText(tty, text string, submit bool) error {
	p, err := t.paneFor(tty)
	if err != nil {
		return err
	}
	if _, err := t.exec("send-keys", "-t", p.SessionID, "-l", "--", text); err != nil {
		return err
	}
	if submit {
		_, err = t.exec("send-keys", "-t", p.SessionID, "Enter")
	}
	return err
}

// Interrupt sends Ctrl-C to the pane owning tty.
func (t *TmuxBackend) Interrupt(tty string) error {
	p, err := t.paneFor(tty)
	if err != nil {
		return err
	}
	_, err = t.exec("send-keys", "-t", p.SessionID, "C-c")
	return err
}

// NewTabAt opens a new window in cwd and leaves prefill unsubmitted on the
// prompt line.
func (t *TmuxBackend) NewTabAt(cwd, prefill string) error {
	out, err := t.exec("new-window", "-c", cwd, "-P", "-F", "#{pane_id}")
	if err != nil {
		return err
	}
	if prefill == "" {
		return nil
	}
	pane := strings.TrimSpace(out)
	_, err = t.exec("send-keys", "-t", pane, "-l", "--", prefill)
	return err
}

// ReopenAt reopens a closed session as a new window: cwd set, resumeCmd left
// unsubmitted.
func (t *TmuxBackend) ReopenAt(cwd, resumeCmd string) error {
	return t.NewTabAt(cwd, resumeCmd)
}
