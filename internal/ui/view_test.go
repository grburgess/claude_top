package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/burgessj/claude_top/internal/registry"
	"github.com/burgessj/claude_top/internal/signalfile"
	"github.com/burgessj/claude_top/internal/transcript"
)

func fixNow(t *testing.T) {
	t.Helper()
	prev := now
	fixed := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	now = func() time.Time { return fixed }
	t.Cleanup(func() { now = prev })
}

func sampleSession() *registry.Session {
	return &registry.Session{
		ID:        "abc123",
		HasSignal: true,
		Signal: signalfile.Signal{
			Type:    "running",
			Project: "claude_top",
			TTY:     "/dev/ttys001",
		},
		Stats: transcript.Stats{
			Title:         "Fix the reducer bugs",
			GitBranch:     "main",
			Model:         "claude-opus-5",
			Effort:        "high",
			ContextTokens: 100_000,
			ContextLimit:  200_000,
			CostUSD:       12.34,
			LastToolCall:  "ZBashZ",
			LastTs:        time.Date(2026, 1, 1, 11, 57, 0, 0, time.UTC),
		},
		LiveAgents: 2,
		State:      registry.StateLive,
	}
}

func TestRenderRowWide(t *testing.T) {
	fixNow(t)
	row := RenderRow(sampleSession(), 120)
	if strings.Contains(row, "\n") {
		t.Fatal("row contains newline")
	}
	if w := lipgloss.Width(row); w > 120 {
		t.Errorf("row width = %d, want <= 120", w)
	}
	for _, want := range []string{"ZBashZ", "opus-5 high", "$12.34", "⚑2", "3m", "50%", "█", "claude_top·main", "Fix the reducer"} {
		if !strings.Contains(row, want) {
			t.Errorf("row missing %q: %q", want, row)
		}
	}
}

func TestRenderRowNarrowDropsToolThenModelThenPct(t *testing.T) {
	fixNow(t)
	s := sampleSession()

	row := RenderRow(s, 95) // < 100: tool dropped, model kept
	if strings.Contains(row, "ZBashZ") {
		t.Errorf("width 95: tool call should be dropped: %q", row)
	}
	if !strings.Contains(row, "opus-5") {
		t.Errorf("width 95: model should be kept: %q", row)
	}

	row = RenderRow(s, 80) // < 85: model dropped, gauge %% kept
	if strings.Contains(row, "opus-5") {
		t.Errorf("width 80: model should be dropped: %q", row)
	}
	if !strings.Contains(row, "50%") {
		t.Errorf("width 80: gauge %% should be kept: %q", row)
	}

	row = RenderRow(s, 65) // < 70: gauge %% dropped, bar kept
	if strings.Contains(row, "%") {
		t.Errorf("width 65: gauge %% should be dropped: %q", row)
	}
	if !strings.Contains(row, "█") {
		t.Errorf("width 65: gauge bar should be kept: %q", row)
	}
}

func TestRenderRowNeverWraps(t *testing.T) {
	fixNow(t)
	sessions := []*registry.Session{
		sampleSession(),
		{ID: "empty"}, // zero-value stats, no signal
		func() *registry.Session {
			s := sampleSession()
			s.Signal.Type = "attention"
			s.Stats.Title = strings.Repeat("very long title ", 20)
			return s
		}(),
		func() *registry.Session {
			s := sampleSession()
			s.State = registry.StateDead
			return s
		}(),
	}
	for _, s := range sessions {
		for _, w := range []int{40, 60, 65, 69, 70, 80, 84, 85, 99, 100, 120, 200} {
			row := RenderRow(s, w)
			if strings.Contains(row, "\n") {
				t.Fatalf("width %d: row wraps", w)
			}
			if got := lipgloss.Width(row); got > w {
				t.Errorf("session %s width %d: rendered %d cols", s.ID, w, got)
			}
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
