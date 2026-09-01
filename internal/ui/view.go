package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/burgessj/claude_top/internal/registry"
)

// now is injectable for tests.
var now = time.Now

// Progressive column-drop thresholds (cols).
const (
	widthTool  = 100 // below: drop tool call
	widthModel = 85  // below: drop model
	widthPct   = 70  // below: drop gauge %
)

// Fixed cell widths.
const (
	glyphW    = 2
	modelW    = 14
	gaugeBarW = 10
	gaugePctW = 5 // " 100%"
	costW     = 8
	agentsW   = 3
	elapsedW  = 4
	toolW     = 14
)

// RenderRow formats one session as a single line no wider than width;
// it never wraps.
func RenderRow(s *registry.Session, width int) string {
	return renderRow(s, width, false)
}

func renderRow(s *registry.Session, width int, flash bool) string {
	showTool := width >= widthTool
	showModel := width >= widthModel
	showPct := width >= widthPct

	glyph, glyphStyle := statusGlyph(s)
	attention := s.HasSignal && s.Signal.Type == "attention" && s.State == registry.StateLive
	if attention && flash {
		glyphStyle = stFlash
	}

	title := s.Stats.Title
	if title == "" {
		title = s.Signal.Project
	}
	if title == "" {
		title = s.ID
	}
	var projParts []string
	if s.Signal.Project != "" {
		projParts = append(projParts, s.Signal.Project)
	}
	if s.Stats.GitBranch != "" {
		projParts = append(projParts, s.Stats.GitBranch)
	}
	proj := strings.Join(projParts, "·")

	model := shortModel(s.Stats.Model)
	if model != "" && s.Stats.Effort != "" {
		model += " " + s.Stats.Effort
	}

	frac := contextFrac(s.Stats.ContextTokens, s.Stats.ContextLimit)
	gauge := gaugeBar(frac, showPct)
	gaugeW := gaugeBarW
	if showPct {
		gaugeW += gaugePctW
	}

	cost := fmt.Sprintf("$%.2f", s.Stats.CostUSD)
	agents := ""
	if s.LiveAgents > 0 {
		agents = fmt.Sprintf("⚑%d", s.LiveAgents)
	}
	elapsed := ""
	if !s.Stats.LastTs.IsZero() {
		elapsed = humanDur(now().Sub(s.Stats.LastTs))
	}
	tool := s.Stats.LastToolCall

	// Flex budget for title + project·branch.
	fixed := glyphW + gaugeW + costW + agentsW + elapsedW
	cells := 7 // glyph, title, proj, gauge, cost, agents, elapsed
	if showModel {
		fixed += modelW
		cells++
	}
	if showTool {
		fixed += toolW
		cells++
	}
	flex := width - fixed - (cells - 1)
	if flex < 0 {
		flex = 0
	}
	titleWd := flex * 3 / 5
	projWd := flex - titleWd

	styleFor := rowStyles(s, attention)
	var parts []string
	add := func(text string, w int, right bool, base lipgloss.Style) {
		if w <= 0 {
			return
		}
		cell := fit(text, w, right)
		parts = append(parts, styleFor(base).Render(cell))
	}
	add(glyph, glyphW, false, glyphStyle)
	add(title, titleWd, false, rowTitle)
	add(proj, projWd, false, rowMeta)
	if showModel {
		add(model, modelW, false, rowModel)
	}
	add(gauge, gaugeW, false, gaugeStyleFor(frac))
	add(cost, costW, true, rowCost)
	add(agents, agentsW, false, rowAgents)
	add(elapsed, elapsedW, true, rowMeta)
	if showTool {
		add(tool, toolW, false, rowTool)
	}
	return strings.Join(parts, " ")
}

// rowStyles picks the per-cell style mapping: live rows keep full color,
// recent rows dim, dead rows dim further; attention rows go bold.
func rowStyles(s *registry.Session, attention bool) func(lipgloss.Style) lipgloss.Style {
	switch {
	case s.State == registry.StateDead:
		return func(lipgloss.Style) lipgloss.Style { return dimmerRow }
	case s.State == registry.StateRecent:
		return func(lipgloss.Style) lipgloss.Style { return dimRow }
	case attention:
		return func(base lipgloss.Style) lipgloss.Style { return base.Bold(true) }
	default:
		return func(base lipgloss.Style) lipgloss.Style { return base }
	}
}

func statusGlyph(s *registry.Session) (string, lipgloss.Style) {
	if s.HasSignal {
		switch s.Signal.Type {
		case "attention":
			return "🔴", stAttention
		case "idle":
			return "💤", stIdle
		case "running":
			return "⚡", stWorking
		}
	}
	if s.State == registry.StateLive {
		return "⚡", stWorking
	}
	return "·", stIdle
}

func contextFrac(tokens, limit int64) float64 {
	if limit <= 0 {
		return 0
	}
	frac := float64(tokens) / float64(limit)
	if frac < 0 {
		return 0
	}
	if frac > 1 {
		return 1
	}
	return frac
}

// gaugeBar renders a 10-cell context mini-bar, optionally with a percent.
func gaugeBar(frac float64, showPct bool) string {
	filled := int(frac*gaugeBarW + 0.5)
	if filled > gaugeBarW {
		filled = gaugeBarW
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", gaugeBarW-filled)
	if showPct {
		bar += fmt.Sprintf(" %3.0f%%", frac*100)
	}
	return bar
}

// shortModel shortens a model id: claude-opus-5 → opus-5,
// claude-haiku-4-5 → haiku-4.5, trailing date stamps dropped.
func shortModel(m string) string {
	if m == "" {
		return ""
	}
	m = strings.TrimPrefix(m, "claude-")
	parts := strings.Split(m, "-")
	if n := len(parts); n > 1 && len(parts[n-1]) == 8 && isDigits(parts[n-1]) {
		parts = parts[:n-1]
	}
	if n := len(parts); n >= 3 && isDigits(parts[n-2]) && isDigits(parts[n-1]) {
		parts[n-2] += "." + parts[n-1]
		parts = parts[:n-1]
	}
	return strings.Join(parts, "-")
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func humanDur(d time.Duration) string {
	switch {
	case d < 0:
		return "0s"
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

// fit truncates plain text to display width w (ellipsis when cut) and pads
// to exactly w columns. Style AFTER fitting: it must not see ANSI codes.
func fit(s string, w int, right bool) string {
	if w <= 0 {
		return ""
	}
	if lipgloss.Width(s) > w {
		var b strings.Builder
		cw := 0
		for _, r := range s {
			rw := lipgloss.Width(string(r))
			if cw+rw > w-1 {
				break
			}
			b.WriteRune(r)
			cw += rw
		}
		s = b.String() + "…"
	}
	pad := strings.Repeat(" ", w-lipgloss.Width(s))
	if right {
		return pad + s
	}
	return s + pad
}

// View renders the full screen: header band, session rows, footer.
func (m Model) View() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}
	header := m.headerView()
	footer := footerStyle.Render(" q quit · j/k move · h view · enter focus · r refresh")

	rowsH := m.rowsHeight()
	lines := make([]string, 0, rowsH)
	end := m.scroll + rowsH
	if end > len(m.rows) {
		end = len(m.rows)
	}
	for i := m.scroll; i < end; i++ {
		prefix := "  "
		if i == m.cursor {
			prefix = cursorStyle.Render("▌ ")
		}
		lines = append(lines, prefix+renderRow(m.rows[i], m.width-2, m.flash))
	}
	if len(m.rows) == 0 {
		lines = append(lines, emptyStyle.Render("  no sessions — press h to widen the view"))
	}
	for len(lines) < rowsH {
		lines = append(lines, "")
	}
	return header + "\n" + strings.Join(lines, "\n") + "\n" + footer
}

func (m Model) headerView() string {
	var working, idle, attn int
	var cost float64
	for _, s := range m.rows {
		if s.State != registry.StateLive {
			continue
		}
		cost += s.Stats.CostUSD
		switch s.Signal.Type {
		case "attention":
			attn++
		case "idle":
			idle++
		default:
			working++
		}
	}
	sep := headerSep.Render(" · ")
	content := appName.Render("claude_top") + "  " +
		stWorking.Render(fmt.Sprintf("⚡%d working", working)) + sep +
		stIdle.Render(fmt.Sprintf("💤%d idle", idle)) + sep +
		stAttention.Render(fmt.Sprintf("🔴%d attention", attn)) + "  " +
		rowMeta.Render(fmt.Sprintf("%d sessions", len(m.rows))) + "  " +
		rowCost.Render(fmt.Sprintf("$%.2f", cost)) + "  " +
		modeLabelStyle.Render("["+modeLabel(m.mode)+"]")
	return headerBox.Width(m.width - 2).Render(content)
}

func modeLabel(mode registry.Mode) string {
	switch mode {
	case registry.ModeLive:
		return "Live"
	case registry.ModeLiveRecent:
		return "Live+Recent"
	default:
		return "All"
	}
}
