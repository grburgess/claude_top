// Package ui is the Bubble Tea list view over the session registry.
package ui

import (
	"sync"
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

// statusExpiry is how long a transient action-error status line stays up.
const statusExpiry = 3 * time.Second

// enrichWorkers bounds the goroutine pool parsing transcripts in the
// background after an index pass.
const enrichWorkers = 4

type tickMsg time.Time

type sigMsg signalfile.Signal

type termTabsMsg struct {
	tabs     map[string]registry.TermTab
	backends map[string]string // tty → backend name
}

// actionErrMsg carries a failed terminal-action error to the status line.
type actionErrMsg struct{ err error }

// enrichedMsg signals that a background EnsureStats batch finished.
type enrichedMsg struct{}

// Model is the Elm-style model for the list view.
type Model struct {
	reg     *registry.Registry
	term    bridge.Backend
	signals <-chan signalfile.Signal

	mode   registry.Mode
	rows   []*registry.Session
	cursor int
	scroll int
	width  int
	height int
	tick   int

	// enriching guards against overlapping background EnsureStats batches.
	enriching bool

	// x-interrupt armed state: session ID and when it was armed.
	armedID string
	armedAt time.Time

	// transient status line for action errors; empty when none.
	statusMsg string
	statusAt  time.Time

	// ttyBackend names the backend owning each tty (from Enumerate).
	ttyBackend map[string]string

	// p prompt minibuffer state.
	prompting   bool
	promptTTY   string
	promptTitle string
	input       textinput.Model
}

// New builds the list-view model; signals may be nil (no watcher).
func New(reg *registry.Registry, term bridge.Backend, signals <-chan signalfile.Signal) Model {
	m := Model{reg: reg, term: term, signals: signals, mode: registry.ModeLive}
	m.refresh()
	return m
}

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{tick(), enumerateTabs(m.term)}
	if m.signals != nil {
		cmds = append(cmds, waitSignal(m.signals))
	}
	if cmd := m.maybeEnrich(); cmd != nil {
		cmds = append(cmds, cmd)
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
// osascript/tmux) and delivers the tty→tab and tty→backend maps as a message.
func enumerateTabs(term bridge.Backend) tea.Cmd {
	if term == nil {
		return nil
	}
	return func() tea.Msg {
		ss, err := term.Enumerate()
		if err != nil {
			return nil
		}
		tabs := make(map[string]registry.TermTab, len(ss))
		backends := make(map[string]string, len(ss))
		for _, s := range ss {
			tabs[s.TTY] = registry.TermTab{WindowID: s.WindowID, TabIndex: s.TabIndex, Backend: s.BackendName}
			backends[s.TTY] = s.BackendName
		}
		return termTabsMsg{tabs: tabs, backends: backends}
	}
}

// actionCmd runs a terminal action off the Update loop and surfaces its
// error (if any) on the status line.
func actionCmd(do func() error) tea.Cmd {
	return func() tea.Msg {
		if err := do(); err != nil {
			return actionErrMsg{err}
		}
		return nil
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.clampView()
	case tickMsg:
		m.tick++
		if m.statusMsg != "" && now().Sub(m.statusAt) > statusExpiry {
			m.statusMsg = ""
		}
		m.refresh()
		cmds := []tea.Cmd{tick()}
		if m.tick%enumerateEvery == 0 {
			cmds = append(cmds, enumerateTabs(m.term))
		}
		if cmd := m.maybeEnrich(); cmd != nil {
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)
	case termTabsMsg:
		m.reg.SetTermTabs(msg.tabs)
		m.ttyBackend = msg.backends
		m.refresh()
	case actionErrMsg:
		m.statusMsg = msg.err.Error()
		m.statusAt = now()
	case sigMsg:
		m.refresh()
		return m, tea.Batch(waitSignal(m.signals), m.maybeEnrich())
	case enrichedMsg:
		m.enriching = false
		m.refresh()
		return m, m.maybeEnrich()
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
			return m, m.maybeEnrich() // lazy-parse the newly selected row
		case key.Matches(msg, keys.Down):
			if m.cursor < len(m.rows)-1 {
				m.cursor++
			}
			m.clampView()
			return m, m.maybeEnrich()
		case key.Matches(msg, keys.Mode):
			m.mode = nextMode(m.mode)
			m.refresh()
			return m, m.maybeEnrich()
		case key.Matches(msg, keys.Refresh):
			m.refresh()
			return m, m.maybeEnrich()
		case key.Matches(msg, keys.Interrupt):
			if s := m.selected(); s != nil && s.Open &&
				s.HasSignal && s.Signal.TTY != "" {
				if m.armedID == s.ID && now().Sub(m.armedAt) <= interruptArmWindow {
					m.armedID = ""
					tty := s.Signal.TTY
					term := m.term
					return m, actionCmd(func() error { return term.Interrupt(tty) })
				}
				m.armedID, m.armedAt = s.ID, now()
			}
		case key.Matches(msg, keys.Prompt):
			if s := m.selected(); s != nil && s.Open &&
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
				// Only a closed session gets a resume tab; an open one is
				// jumped to, however long it has been idle.
				if s.Open {
					if s.Signal.TTY != "" {
						tty := s.Signal.TTY
						return m, actionCmd(func() error { return term.FocusTTY(tty) })
					}
					break
				}
				// Live without a probeable pid (no plugin): the tab is open
				// somewhere but unroutable — reopening would duplicate it.
				if s.State == registry.StateLive || s.Signal.Cwd == "" {
					break
				}
				cwd, id := s.Signal.Cwd, s.ID
				return m, actionCmd(func() error { return term.ReopenAt(cwd, "claude -r "+id) })
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
		return m, actionCmd(func() error { return term.SendText(tty, text, true) })
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

// refresh runs the cheap index pass only; transcript parsing happens in the
// background via maybeEnrich so first paint never waits on a Tail.
func (m *Model) refresh() {
	if m.reg == nil { // tests drive Model without a registry
		m.clampView()
		return
	}
	_ = m.reg.RefreshIndex()
	m.rows = m.reg.List(m.mode)
	m.clampView()
}

// pendingStats lists session IDs still awaiting EnsureStats: the selected
// row first, then listed rows in view order from the scroll position
// (visible first), then any unlisted live+recent sessions. Dead sessions
// only appear in rows under ModeAll, so history transcripts are parsed
// lazily — when [all] is entered or a row is selected.
func (m Model) pendingStats() []string {
	var ids []string
	queued := map[string]bool{}
	add := func(s *registry.Session) {
		if s.StatsReady || queued[s.ID] {
			return
		}
		queued[s.ID] = true
		ids = append(ids, s.ID)
	}
	if sel := m.selected(); sel != nil {
		add(sel)
	}
	for i := 0; i < len(m.rows); i++ {
		add(m.rows[(m.scroll+i)%len(m.rows)])
	}
	for _, s := range m.reg.List(registry.ModeLiveRecent) {
		add(s) // live+recent parse immediately regardless of view mode
	}
	return ids
}

// maybeEnrich starts one background EnsureStats batch over the pending
// rows, bounded to enrichWorkers goroutines. The registry is mutex-guarded,
// so workers call EnsureStats directly; the returned enrichedMsg just
// triggers a re-render.
func (m *Model) maybeEnrich() tea.Cmd {
	if m.reg == nil || m.enriching {
		return nil
	}
	ids := m.pendingStats()
	if len(ids) == 0 {
		return nil
	}
	m.enriching = true
	reg := m.reg
	return func() tea.Msg {
		sem := make(chan struct{}, enrichWorkers)
		var wg sync.WaitGroup
		for _, id := range ids {
			wg.Add(1)
			sem <- struct{}{}
			go func(id string) {
				defer wg.Done()
				reg.EnsureStats(id)
				<-sem
			}(id)
		}
		wg.Wait()
		return enrichedMsg{}
	}
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
