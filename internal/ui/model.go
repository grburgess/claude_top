// Package ui is the Bubble Tea list view over the session registry.
package ui

import (
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/burgessj/claude_top/internal/bridge"
	"github.com/burgessj/claude_top/internal/registry"
	"github.com/burgessj/claude_top/internal/signalfile"
)

// enumerateEvery is the tick period between iTerm tab re-enumerations.
const enumerateEvery = 5

// interruptArmWindow is how long an `x` press stays armed for the second
// confirming press.
const interruptArmWindow = 3 * time.Second

type tickMsg time.Time

type sigMsg signalfile.Signal

type termTabsMsg map[string]registry.TermTab

// Model is the Elm-style model for the list view.
type Model struct {
	reg     *registry.Registry
	term    bridge.ITerm
	signals <-chan signalfile.Signal

	mode   registry.Mode
	rows   []*registry.Session
	cursor int
	scroll int
	width  int
	height int
	tick   int

	// x-interrupt armed state: session ID and when it was armed.
	armedID string
	armedAt time.Time

	// p prompt minibuffer state.
	prompting   bool
	promptTTY   string
	promptTitle string
	input       textinput.Model
}

// New builds the list-view model; signals may be nil (no watcher).
func New(reg *registry.Registry, term bridge.ITerm, signals <-chan signalfile.Signal) Model {
	m := Model{reg: reg, term: term, signals: signals, mode: registry.ModeLive}
	m.refresh()
	return m
}

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{tick(), enumerateTabs(m.term)}
	if m.signals != nil {
		cmds = append(cmds, waitSignal(m.signals))
	}
	return tea.Batch(cmds...)
}

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func waitSignal(ch <-chan signalfile.Signal) tea.Cmd {
	return func() tea.Msg {
		s, ok := <-ch
		if !ok {
			return nil
		}
		return sigMsg(s)
	}
}

// enumerateTabs runs bridge.Enumerate off the Update loop (it execs
// osascript) and delivers the tty→tab map as a message.
func enumerateTabs(term bridge.ITerm) tea.Cmd {
	if term == nil {
		return nil
	}
	return func() tea.Msg {
		ss, err := term.Enumerate()
		if err != nil {
			return nil
		}
		tabs := make(map[string]registry.TermTab, len(ss))
		for _, s := range ss {
			tabs[s.TTY] = registry.TermTab{WindowID: s.WindowID, TabIndex: s.TabIndex}
		}
		return termTabsMsg(tabs)
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.clampView()
	case tickMsg:
		m.tick++
		m.refresh()
		cmds := []tea.Cmd{tick()}
		if m.tick%enumerateEvery == 0 {
			cmds = append(cmds, enumerateTabs(m.term))
		}
		return m, tea.Batch(cmds...)
	case termTabsMsg:
		m.reg.SetTermTabs(msg)
		m.refresh()
	case sigMsg:
		m.refresh()
		return m, waitSignal(m.signals)
	case tea.KeyMsg:
		if m.prompting {
			return m.updatePrompt(msg)
		}
		switch {
		case key.Matches(msg, keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, keys.Up):
			if m.cursor > 0 {
				m.cursor--
			}
			m.clampView()
		case key.Matches(msg, keys.Down):
			if m.cursor < len(m.rows)-1 {
				m.cursor++
			}
			m.clampView()
		case key.Matches(msg, keys.Mode):
			m.mode = nextMode(m.mode)
			m.refresh()
		case key.Matches(msg, keys.Refresh):
			m.refresh()
		case key.Matches(msg, keys.Interrupt):
			if s := m.selected(); s != nil && s.State == registry.StateLive &&
				s.HasSignal && s.Signal.TTY != "" {
				if m.armedID == s.ID && now().Sub(m.armedAt) <= interruptArmWindow {
					m.armedID = ""
					tty := s.Signal.TTY
					term := m.term
					return m, func() tea.Msg { _ = term.Interrupt(tty); return nil }
				}
				m.armedID, m.armedAt = s.ID, now()
			}
		case key.Matches(msg, keys.Prompt):
			if s := m.selected(); s != nil && s.State == registry.StateLive &&
				s.HasSignal && s.Signal.TTY != "" {
				m.prompting = true
				m.promptTTY = s.Signal.TTY
				m.promptTitle = titleOf(s)
				m.input = textinput.New()
				m.input.Focus()
			}
		case key.Matches(msg, keys.Focus):
			if s := m.selected(); s != nil {
				term := m.term
				if s.State == registry.StateLive {
					if s.HasSignal && s.Signal.TTY != "" {
						tty := s.Signal.TTY
						return m, func() tea.Msg { _ = term.FocusTTY(tty); return nil }
					}
					break
				}
				cwd, id := s.Signal.Cwd, s.ID
				return m, func() tea.Msg { _ = term.ReopenAt(cwd, "claude -r "+id); return nil }
			}
		}
	}
	return m, nil
}

// updatePrompt handles keys while the prompt minibuffer is open: enter sends
// the text to the session, esc cancels, everything else edits the input.
func (m Model) updatePrompt(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.prompting = false
		return m, nil
	case tea.KeyEnter:
		m.prompting = false
		text := m.input.Value()
		if text == "" {
			return m, nil
		}
		tty := m.promptTTY
		term := m.term
		return m, func() tea.Msg { _ = term.SendText(tty, text, true); return nil }
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// interruptArmed reports whether the armed x-interrupt applies to s and has
// not expired.
func (m Model) interruptArmed(s *registry.Session) bool {
	return s != nil && m.armedID == s.ID && now().Sub(m.armedAt) <= interruptArmWindow
}

func nextMode(mode registry.Mode) registry.Mode {
	switch mode {
	case registry.ModeLive:
		return registry.ModeLiveRecent
	case registry.ModeLiveRecent:
		return registry.ModeAll
	default:
		return registry.ModeLive
	}
}

func (m *Model) refresh() {
	_ = m.reg.Refresh()
	m.rows = m.reg.List(m.mode)
	m.clampView()
}

// clampView keeps the cursor in range; View slides the block window itself
// (block heights vary), so scroll only needs to stay at or above the cursor.
func (m *Model) clampView() {
	if m.cursor >= len(m.rows) {
		m.cursor = len(m.rows) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.scroll > m.cursor {
		m.scroll = m.cursor
	}
	if m.scroll < 0 {
		m.scroll = 0
	}
}

func (m Model) selected() *registry.Session {
	if m.cursor < 0 || m.cursor >= len(m.rows) {
		return nil
	}
	return m.rows[m.cursor]
}
