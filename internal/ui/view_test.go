package ui

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/burgessj/claude_top/internal/registry"
	"github.com/burgessj/claude_top/internal/signalfile"
	"github.com/burgessj/claude_top/internal/transcript"
)

// TestMain pins a color profile: tests run without a TTY, where lipgloss
// would otherwise strip all styling (making pulse/tint assertions no-ops).
func TestMain(m *testing.M) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	os.Exit(m.Run())
}

func fixNow(t *testing.T) {
	t.Helper()
	prev := now
	fixed := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	now = func() time.Time { return fixed }
	t.Cleanup(func() { now = prev })
}

func sampleSession() *registry.Session {
	return &registry.Session{
		ID:         "abc123",
		HasSignal:  true,
		StatsReady: true,
		Signal: signalfile.Signal{
			Type:    "running",
			Project: "claude_top",
			Cwd:     "/Users/x/claude_top",
			TTY:     "/dev/ttys001",
		},
		Stats: transcript.Stats{
			Title:             "Fix the reducer bugs",
			GitBranch:         "main",
			Model:             "claude-opus-5",
			Effort:            "high",
			Mode:              "chat",
			PermissionMode:    "default",
			ContextTokens:     285_000,
			ContextLimit:      1_000_000,
			CostUSD:           64.19,
			Turns:             42,
			Skills:            []string{"pull-request", "code-review"},
			MCPServers:        []string{"github"},
			TokensPerTurn:     []int64{10, 200, 50, 800, 400},
			LastAssistantText: "Done. Both projects are in the graph and the rewire landed — 2 nodes moved and every edge survived the migration intact.",
			FirstTs:           time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC),
			LastTs:            time.Date(2026, 1, 1, 11, 57, 0, 0, time.UTC),
		},
		LiveAgents:  2,
		TotalAgents: 5,
		AgentNames:  []string{"scout", "fixer"},
		State:       registry.StateLive,
		TabIndex:    6,
		HasTab:      true,
	}
}

func idleSession() *registry.Session {
	s := sampleSession()
	s.Signal.Type = "idle"
	return s
}

func blockLinesOf(t *testing.T, s *registry.Session, width int, selected bool, tick int) []string {
	t.Helper()
	block := RenderBlock(s, width, selected, tick, "")
	lines := strings.Split(block, "\n")
	for _, ln := range lines {
		if w := lipgloss.Width(ln); w > width {
			t.Errorf("line %d cols, want <= %d: %q", w, width, ln)
		}
	}
	return lines
}

func TestRenderBlockWorkingUnselected(t *testing.T) {
	fixNow(t)
	lines := blockLinesOf(t, sampleSession(), 100, false, 0)
	if len(lines) != 2 {
		t.Fatalf("working block = %d lines, want 2 (no snippet)", len(lines))
	}
	l1, l2 := lines[0], lines[1]
	for _, want := range []string{"●", "Chat", "◐", "Fix the reducer bugs", "⌘6"} {
		if !strings.Contains(l1, want) {
			t.Errorf("line1 missing %q: %q", want, l1)
		}
	}
	if !strings.Contains(l2, "working") {
		t.Errorf("line2 missing state word: %q", l2)
	}
	if strings.Contains(l1+l2, "Done.") {
		t.Error("working block must not show the assistant snippet")
	}
}

func TestRenderBlockSpinnerRotates(t *testing.T) {
	fixNow(t)
	s := sampleSession()
	for tick, glyph := range spinnerFrames {
		if b := RenderBlock(s, 100, false, tick, ""); !strings.Contains(b, glyph) {
			t.Errorf("tick %d: missing spinner frame %q", tick, glyph)
		}
	}
}

func TestRenderBlockIdleShowsSnippet(t *testing.T) {
	fixNow(t)
	lines := blockLinesOf(t, idleSession(), 100, false, 0)
	if len(lines) < 3 || len(lines) > 4 {
		t.Fatalf("idle block = %d lines, want 3-4", len(lines))
	}
	body := strings.Join(lines, "\n")
	if !strings.Contains(body, "idle") {
		t.Errorf("missing state word: %q", body)
	}
	if !strings.Contains(body, "Done. Both projects") {
		t.Errorf("missing assistant snippet: %q", body)
	}
	if !strings.Contains(body, "✳") {
		t.Errorf("missing idle glyph: %q", body)
	}
}

func TestRenderBlockUnselectedAtMostFourLines(t *testing.T) {
	fixNow(t)
	for _, s := range []*registry.Session{
		sampleSession(),
		idleSession(),
		func() *registry.Session { s := sampleSession(); s.Signal.Type = "attention"; return s }(),
		{ID: "empty"},
	} {
		if n := len(blockLinesOf(t, s, 100, false, 0)); n > 4 {
			t.Errorf("session %s: unselected block = %d lines, want <= 4", s.ID, n)
		}
	}
}

func TestRenderBlockSelectedDetailWide(t *testing.T) {
	fixNow(t)
	lines := blockLinesOf(t, idleSession(), 100, true, 0)
	body := strings.Join(lines, "\n")
	for _, want := range []string{
		"▍",       // accent bar
		"285k/1M", // context text
		"(28%)",   // context percent
		"█",       // gauge cells
		"$64.19",  // cost
		"opus-5 high",
		"claude_top · main",
		"default",                   // permission mode
		"pull-request, code-review", // skills
		"github",                    // mcp
		"⚑2 live / 5 total",         // subagents
		"scout, fixer",
		"42 turns",
		"1h", // duration
		"╭",  // card border
	} {
		if !strings.Contains(body, want) {
			t.Errorf("selected block missing %q:\n%s", want, body)
		}
	}
	// sparkline present with a max-height cell for the 800 turn
	if !strings.Contains(body, "█▄") && !strings.Contains(body, "▁") {
		t.Errorf("selected block missing sparkline:\n%s", body)
	}
}

func TestRenderBlockSelectedNarrowStacks(t *testing.T) {
	fixNow(t)
	wideN := len(blockLinesOf(t, idleSession(), 120, true, 0))
	narrowN := len(blockLinesOf(t, idleSession(), 80, true, 0))
	if narrowN <= wideN {
		t.Errorf("narrow selected block should stack columns: wide=%d narrow=%d lines", wideN, narrowN)
	}
}

func TestRenderBlockNeverWraps(t *testing.T) {
	fixNow(t)
	sessions := []*registry.Session{
		sampleSession(),
		idleSession(),
		{ID: "empty"}, // zero-value stats, no signal
		func() *registry.Session {
			s := sampleSession()
			s.Signal.Type = "attention"
			s.Stats.Title = strings.Repeat("very long title ", 20)
			s.Stats.LastAssistantText = strings.Repeat("word ", 200)
			return s
		}(),
		func() *registry.Session {
			s := sampleSession()
			s.State = registry.StateDead
			return s
		}(),
		func() *registry.Session {
			s := sampleSession()
			s.State = registry.StateRecent
			return s
		}(),
	}
	for _, s := range sessions {
		for _, w := range []int{20, 40, 60, 80, 89, 90, 100, 120, 200} {
			for _, sel := range []bool{false, true} {
				blockLinesOf(t, s, w, sel, 1) // width assertions inside
			}
		}
	}
}

func TestRenderBlockNoTabOmitsShortcut(t *testing.T) {
	fixNow(t)
	s := sampleSession()
	s.HasTab = false
	if b := RenderBlock(s, 100, false, 0, ""); strings.Contains(b, "⌘") {
		t.Errorf("block shows ⌘ without a tab: %q", b)
	}
}

func TestAttentionDotPulses(t *testing.T) {
	fixNow(t)
	s := sampleSession()
	s.Signal.Type = "attention"
	if RenderBlock(s, 100, false, 0, "") == RenderBlock(s, 100, false, 1, "") {
		t.Error("attention block identical on odd/even ticks, want pulse")
	}
}

func TestRenderBlockWhereLineShowsBackend(t *testing.T) {
	fixNow(t)
	b := RenderBlock(idleSession(), 120, true, 0, "tmux")
	if !strings.Contains(b, "/Users/x/claude_top · tmux") {
		t.Errorf("where line missing backend name:\n%s", b)
	}
	b = RenderBlock(idleSession(), 120, true, 0, "")
	if strings.Contains(b, "· tmux") || strings.Contains(b, "· iterm") {
		t.Errorf("unknown backend must not be named:\n%s", b)
	}
}

func TestWrapWords(t *testing.T) {
	got := wrapWords("one two three four five six", 10, 2)
	if len(got) != 2 {
		t.Fatalf("lines = %v", got)
	}
	if got[0] != "one two" {
		t.Errorf("line1 = %q", got[0])
	}
	if !strings.HasSuffix(got[1], "…") {
		t.Errorf("line2 not ellipsized: %q", got[1])
	}
}

func TestSparkline(t *testing.T) {
	if got := sparkline([]int64{0, 400, 800}, 40); got != "▁▄█" {
		t.Errorf("sparkline = %q, want ▁▄█", got)
	}
	if got := sparkline(nil, 40); got != "—" {
		t.Errorf("sparkline(nil) = %q", got)
	}
}

func TestHumanTokens(t *testing.T) {
	cases := map[int64]string{
		999:       "999",
		285_000:   "285k",
		1_000_000: "1M",
		1_500_000: "1.5M",
	}
	for in, want := range cases {
		if got := humanTokens(in); got != want {
			t.Errorf("humanTokens(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestShortModel(t *testing.T) {
	cases := map[string]string{
		"claude-opus-5":            "opus-5",
		"claude-fable-5":           "fable-5",
		"claude-haiku-4-5":         "haiku-4.5",
		"claude-sonnet-4-20250514": "sonnet-4",
		"":                         "",
	}
	for in, want := range cases {
		if got := shortModel(in); got != want {
			t.Errorf("shortModel(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestHumanDur(t *testing.T) {
	cases := map[time.Duration]string{
		30 * time.Second: "30s",
		3 * time.Minute:  "3m",
		2 * time.Hour:    "2h",
		49 * time.Hour:   "2d",
	}
	for in, want := range cases {
		if got := humanDur(in); got != want {
			t.Errorf("humanDur(%v) = %q, want %q", in, got, want)
		}
	}
}

func TestTabLabelPerBackend(t *testing.T) {
	s := sampleSession()
	s.HasTab, s.TabIndex = true, 3
	s.TabBackend = "tmux"
	if b := RenderBlock(s, 100, false, 0, ""); !strings.Contains(b, "⊞3") {
		t.Errorf("tmux tab should render ⊞3: %q", b)
	}
	s.TabBackend = "iterm"
	if b := RenderBlock(s, 100, false, 0, ""); !strings.Contains(b, "⌘3") {
		t.Errorf("iterm tab should render ⌘3: %q", b)
	}
}
