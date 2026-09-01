package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/burgessj/claude_top/internal/bridge"
	"github.com/burgessj/claude_top/internal/registry"
)

// setNow overrides the view clock with a mutable pointer for arm-expiry tests.
func setNow(t *testing.T, at *time.Time) {
	t.Helper()
	prev := now
	now = func() time.Time { return *at }
	t.Cleanup(func() { now = prev })
}

func testModel(sessions ...*registry.Session) (Model, *bridge.MockITerm) {
	mock := &bridge.MockITerm{}
	return Model{term: mock, rows: sessions, mode: registry.ModeLive, width: 100, height: 40}, mock
}

func keyRunes(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func press(t *testing.T, m Model, msg tea.Msg) (Model, tea.Cmd) {
	t.Helper()
	m2, cmd := m.Update(msg)
	return m2.(Model), cmd
}

func runCmd(cmd tea.Cmd) {
	if cmd != nil {
		cmd()
	}
}

func TestInterruptRequiresDoublePress(t *testing.T) {
	at := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	setNow(t, &at)
	m, mock := testModel(sampleSession())

	m, cmd := press(t, m, keyRunes("x"))
	runCmd(cmd)
	if len(mock.Calls) != 0 {
		t.Fatalf("first x must not interrupt, calls = %v", mock.Calls)
	}
	if !strings.Contains(m.View(), "press x again to interrupt") {
		t.Error("armed state not shown in the selected block")
	}

	_, cmd = press(t, m, keyRunes("x"))
	if cmd == nil {
		t.Fatal("second x within window returned no cmd")
	}
	runCmd(cmd)
	if len(mock.Calls) != 1 || mock.Calls[0] != "Interrupt(/dev/ttys001)" {
		t.Errorf("calls = %v, want [Interrupt(/dev/ttys001)]", mock.Calls)
	}
}

func TestInterruptArmExpires(t *testing.T) {
	at := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	setNow(t, &at)
	m, mock := testModel(sampleSession())

	m, cmd := press(t, m, keyRunes("x"))
	runCmd(cmd)
	at = at.Add(interruptArmWindow + time.Second) // window expired
	m, cmd = press(t, m, keyRunes("x"))           // re-arms, no interrupt
	runCmd(cmd)
	if len(mock.Calls) != 0 {
		t.Errorf("expired arm must re-arm, not interrupt: calls = %v", mock.Calls)
	}
	if !m.interruptArmed(m.rows[0]) {
		t.Error("second press after expiry should re-arm")
	}
}

func TestInterruptIgnoredForNonLive(t *testing.T) {
	at := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	setNow(t, &at)
	s := sampleSession()
	s.State = registry.StateDead
	m, mock := testModel(s)
	m, cmd := press(t, m, keyRunes("x"))
	runCmd(cmd)
	_, cmd = press(t, m, keyRunes("x"))
	runCmd(cmd)
	if len(mock.Calls) != 0 {
		t.Errorf("dead session must never interrupt: calls = %v", mock.Calls)
	}
}

func TestActionErrorSurfacesOnStatusLine(t *testing.T) {
	at := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	setNow(t, &at)
	m, mock := testModel(sampleSession())
	mock.Err = bridge.ErrUnsupported
	_, cmd := press(t, m, tea.KeyMsg{Type: tea.KeyEnter}) // FocusTTY fails
	if cmd == nil {
		t.Fatal("enter returned no cmd")
	}
	msg := cmd()
	aem, ok := msg.(actionErrMsg)
	if !ok {
		t.Fatalf("cmd msg = %T, want actionErrMsg", msg)
	}
	m, _ = press(t, m, aem)
	if !strings.Contains(m.View(), "unsupported") {
		t.Error("action error missing from status line")
	}
	// Expires after statusExpiry on the next tick.
	at = at.Add(statusExpiry + time.Second)
	m, _ = press(t, m, tickMsg(at))
	if strings.Contains(m.View(), "unsupported") {
		t.Error("status line did not expire")
	}
}

func TestPromptDispatchSendsText(t *testing.T) {
	m, mock := testModel(sampleSession())
	m, _ = press(t, m, keyRunes("p"))
	if !m.prompting {
		t.Fatal("p on live session must open the prompt minibuffer")
	}
	if !strings.Contains(m.View(), "prompt → Fix the reducer bugs") {
		t.Error("minibuffer title missing from footer")
	}
	m, _ = press(t, m, keyRunes("run tests"))
	_, cmd := press(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter returned no cmd")
	}
	runCmd(cmd)
	want := `SendText(/dev/ttys001,"run tests",true)`
	if len(mock.Calls) != 1 || mock.Calls[0] != want {
		t.Errorf("calls = %v, want [%s]", mock.Calls, want)
	}
}

func TestPromptEscCancels(t *testing.T) {
	m, mock := testModel(sampleSession())
	m, _ = press(t, m, keyRunes("p"))
	m, _ = press(t, m, keyRunes("oops"))
	m, cmd := press(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	runCmd(cmd)
	if m.prompting {
		t.Error("esc must close the minibuffer")
	}
	if len(mock.Calls) != 0 {
		t.Errorf("esc must not send: calls = %v", mock.Calls)
	}
}

func TestPromptIgnoredForNonLive(t *testing.T) {
	s := sampleSession()
	s.State = registry.StateRecent
	m, _ := testModel(s)
	m, _ = press(t, m, keyRunes("p"))
	if m.prompting {
		t.Error("p on non-live session must not open the minibuffer")
	}
}

func TestDeadSessionEnterReopens(t *testing.T) {
	s := sampleSession()
	s.State = registry.StateDead
	m, mock := testModel(s)
	_, cmd := press(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter on dead session returned no cmd")
	}
	runCmd(cmd)
	want := `ReopenAt(/Users/x/claude_top,"claude -r abc123")`
	if len(mock.Calls) != 1 || mock.Calls[0] != want {
		t.Errorf("calls = %v, want [%s]", mock.Calls, want)
	}
}

func TestLiveSessionEnterStillFocuses(t *testing.T) {
	m, mock := testModel(sampleSession())
	_, cmd := press(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter on live session returned no cmd")
	}
	runCmd(cmd)
	if len(mock.Calls) != 1 || mock.Calls[0] != "FocusTTY(/dev/ttys001)" {
		t.Errorf("calls = %v, want [FocusTTY(/dev/ttys001)]", mock.Calls)
	}
}
