// Package ui is the Bubble Tea list view over the session registry.
package ui

import (
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/burgessj/claude_top/internal/bridge"
	"github.com/burgessj/claude_top/internal/registry"
	"github.com/burgessj/claude_top/internal/signalfile"
)

type tickMsg time.Time

type sigMsg signalfile.Signal

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
	flash  bool
}

// New builds the list-view model; signals may be nil (no watcher).
func New(reg *registry.Registry, term bridge.ITerm, signals <-chan signalfile.Signal) Model {
	m := Model{reg: reg, term: term, signals: signals, mode: registry.ModeLive}
	m.refresh()
	return m
}

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{tick()}
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

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.clampView()
	case tickMsg:
		m.flash = !m.flash
		m.refresh()
		return m, tick()
	case sigMsg:
		m.refresh()
		return m, waitSignal(m.signals)
	case tea.KeyMsg:
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
		case key.Matches(msg, keys.Focus):
			if s := m.selected(); s != nil && s.State == registry.StateLive &&
				s.HasSignal && s.Signal.TTY != "" {
				tty := s.Signal.TTY
				term := m.term
				return m, func() tea.Msg { _ = term.FocusTTY(tty); return nil }
			}
		}
	}
	return m, nil
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

// clampView keeps the cursor in range and the scroll window over it.
func (m *Model) clampView() {
	if m.cursor >= len(m.rows) {
		m.cursor = len(m.rows) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	h := m.rowsHeight()
	if m.cursor < m.scroll {
		m.scroll = m.cursor
	}
	if m.cursor >= m.scroll+h {
		m.scroll = m.cursor - h + 1
	}
	if m.scroll < 0 {
		m.scroll = 0
	}
}

// rowsHeight is the visible row count: header box (3) + footer (1).
func (m Model) rowsHeight() int {
	h := m.height - 4
	if h < 1 {
		h = 1
	}
	return h
}

func (m Model) selected() *registry.Session {
	if m.cursor < 0 || m.cursor >= len(m.rows) {
		return nil
	}
	return m.rows[m.cursor]
}
