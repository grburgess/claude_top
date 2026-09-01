// Package bridge drives iTerm2 via osascript.
package bridge

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// TermSession is one iTerm2 session (pane) discovered by Enumerate.
type TermSession struct {
	WindowID  string
	TabIndex  int
	SessionID string
	TTY       string
	Name      string
}

// ITerm is the terminal control surface.
type ITerm interface {
	Enumerate() ([]TermSession, error)
	FocusTTY(tty string) error
	SendText(tty, text string, submit bool) error
	Interrupt(tty string) error
	NewTabAt(cwd, prefill string) error
	ReopenAt(cwd, resumeCmd string) error
}

// OSAITerm implements ITerm with osascript. The run field is injectable
// for tests; nil means exec osascript.
type OSAITerm struct {
	run func(script string) (string, error)
}

// NewOSAITerm returns the real osascript-backed implementation.
func NewOSAITerm() *OSAITerm { return &OSAITerm{} }

func (o *OSAITerm) exec(script string) (string, error) {
	if o.run != nil {
		return o.run(script)
	}
	out, err := exec.Command("osascript", "-e", script).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("osascript: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

// escapeOSA escapes backslashes and double quotes for interpolation into
// an AppleScript string literal.
func escapeOSA(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}

const enumerateScript = `set out to ""
tell application "iTerm2"
	repeat with w in windows
		set tabIdx to 0
		repeat with t in tabs of w
			set tabIdx to tabIdx + 1
			repeat with s in sessions of t
				set out to out & (id of w) & tab & tabIdx & tab & (id of s) & tab & (tty of s) & tab & (name of s) & linefeed
			end repeat
		end repeat
	end repeat
end tell
return out`

// Enumerate lists all iTerm2 sessions across windows and tabs.
func (o *OSAITerm) Enumerate() ([]TermSession, error) {
	out, err := o.exec(enumerateScript)
	if err != nil {
		return nil, err
	}
	return parseEnumerate(out), nil
}

func parseEnumerate(out string) []TermSession {
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
		idx, _ := strconv.Atoi(parts[1])
		sessions = append(sessions, TermSession{
			WindowID:  parts[0],
			TabIndex:  idx,
			SessionID: parts[2],
			TTY:       parts[3],
			Name:      parts[4],
		})
	}
	return sessions
}

// byTTY wraps body in a repeat that binds s (session) and t (tab) for the
// session whose tty matches.
func byTTY(tty, body string) string {
	return fmt.Sprintf(`tell application "iTerm2"
	repeat with w in windows
		repeat with t in tabs of w
			repeat with s in sessions of t
				if tty of s is "%s" then
%s
					return
				end if
			end repeat
		end repeat
	end repeat
end tell`, escapeOSA(tty), body)
}

// FocusTTY selects the tab and session owning tty and activates iTerm2.
func (o *OSAITerm) FocusTTY(tty string) error {
	body := "\t\t\t\t\tselect t\n\t\t\t\t\tselect s\n\t\t\t\t\tactivate"
	_, err := o.exec(byTTY(tty, body))
	return err
}

// SendText writes text to the session owning tty; submit=false leaves it
// unsubmitted on the prompt line.
func (o *OSAITerm) SendText(tty, text string, submit bool) error {
	var body string
	if submit {
		body = fmt.Sprintf("\t\t\t\t\twrite s text \"%s\"", escapeOSA(text))
	} else {
		body = fmt.Sprintf("\t\t\t\t\twrite s text \"%s\" newline NO", escapeOSA(text))
	}
	_, err := o.exec(byTTY(tty, body))
	return err
}

// Interrupt sends Ctrl-C (ASCII 3) to the session owning tty.
func (o *OSAITerm) Interrupt(tty string) error {
	body := "\t\t\t\t\twrite s text (string id 3) newline NO"
	_, err := o.exec(byTTY(tty, body))
	return err
}

// NewTabAt opens a new tab, cds into cwd (submitted), then leaves prefill
// unsubmitted on the prompt line.
func (o *OSAITerm) NewTabAt(cwd, prefill string) error {
	script := fmt.Sprintf(`tell application "iTerm2"
	tell current window
		create tab with default profile
	end tell
	tell current session of current window
		write text "cd %s"
		write text "%s" newline NO
	end tell
end tell`, escapeOSA(cwd), escapeOSA(prefill))
	_, err := o.exec(script)
	return err
}

// ReopenAt reopens a closed session: a new tab with the default profile in
// the current window, a submitted `cd cwd`, then resumeCmd left unsubmitted
// on the prompt line.
func (o *OSAITerm) ReopenAt(cwd, resumeCmd string) error {
	return o.NewTabAt(cwd, resumeCmd)
}
