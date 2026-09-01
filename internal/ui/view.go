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

const (
	gutterW      = 2  // "▍ " / "  " left gutter on every block line
	detailGaugeW = 24 // context gauge cells in the detail card
	twoColMinW   = 90 // detail card goes two-column at/above this width
	snippetLines = 2  // max wrapped lines of LastAssistantText
	sparkTurns   = 40 // sparkline covers the last N turns
)

var spinnerFrames = []string{"◐", "◓", "◑", "◒"}

// blockPhase classifies a session for block rendering.
type blockPhase int

const (
	phaseWorking blockPhase = iota
	phaseIdle
	phaseAttention
	phaseRecent
	phaseDead
)

func phaseOf(s *registry.Session) blockPhase {
	switch s.State {
	case registry.StateDead:
		return phaseDead
	case registry.StateRecent:
		return phaseRecent
	}
	if s.HasSignal {
		switch s.Signal.Type {
		case "attention":
			return phaseAttention
		case "idle":
			return phaseIdle
		}
	}
	return phaseWorking
}

// RenderBlock renders one session as a multi-line toolbelt-style block,
// never wider than width. Selected blocks gain a left accent bar, a faint
// background tint and an inline bordered detail card.
func RenderBlock(s *registry.Session, width int, selected bool, tick int) string {
	cw := width - gutterW
	if cw < 10 {
		cw = 10
	}
	phase := phaseOf(s)
	lines := []string{blockLine1(s, cw, selected, tick, phase), blockLine2(cw, selected, phase)}
	if !selected && (phase == phaseIdle || phase == phaseAttention) {
		lines = append(lines, snippetLinesFor(s, cw, false)...)
	}
	if selected {
		lines = append(lines, snippetLinesFor(s, cw, true)...)
		lines = append(lines, detailCard(s, cw)...)
	}
	for i, ln := range lines {
		lines[i] = gutter(selected) + ln
	}
	return strings.Join(lines, "\n")
}

func gutter(selected bool) string {
	if selected {
		return selBar.Render("▍ ")
	}
	return "  "
}

// blockLine1: dot, mode word, phase glyph, title, right-aligned ⌘N.
func blockLine1(s *registry.Session, cw int, sel bool, tick int, phase blockPhase) string {
	dot, dotStyle := "●", stDim
	glyph, glyphStyle := "✳", stDim
	switch phase {
	case phaseWorking:
		dotStyle = stWorking
		glyph, glyphStyle = spinnerFrames[tick%len(spinnerFrames)], stWorking
	case phaseIdle:
		dotStyle = stIdle
	case phaseAttention:
		dotStyle = stAttention
		if tick%2 == 1 {
			dotStyle = stAttention.Reverse(true) // pulse
		}
		glyph, glyphStyle = "●", stAttention
	case phaseDead:
		dot, dotStyle = "○", stDimmer
		glyph, glyphStyle = "·", stDimmer
	}

	mode := modeWord(s.Stats.Mode)
	title := s.Stats.Title
	if title == "" {
		title = s.Signal.Project
	}
	if title == "" {
		title = s.ID
	}
	tab := ""
	if s.HasTab {
		tab = fmt.Sprintf("⌘%d", s.TabIndex)
	}

	// dot(1) sp mode sp glyph(1) sp title [sp tab]
	fixed := 1 + 1 + lipgloss.Width(mode) + 1 + 1 + 1
	titleW := cw - fixed
	if tab != "" {
		titleW -= lipgloss.Width(tab) + 1
	}
	sp := tinted(lipgloss.NewStyle(), sel).Render(" ")
	out := tinted(dotStyle, sel).Render(dot) + sp +
		tinted(blockMode, sel).Render(mode) + sp +
		tinted(glyphStyle, sel).Render(glyph) + sp +
		tinted(blockTitle, sel).Render(fit(title, titleW, false))
	if tab != "" {
		out += sp + tinted(blockTab, sel).Render(tab)
	}
	return out
}

// blockLine2: the state word alone, colored, indented under the mode word.
func blockLine2(cw int, sel bool, phase blockPhase) string {
	word, style := "idle", stDim
	switch phase {
	case phaseWorking:
		word, style = "working", stWorking
	case phaseAttention:
		word, style = "attention", stAttention
	case phaseRecent:
		word, style = "recent", stDim
	case phaseDead:
		word, style = "dead", stDimmer
	}
	return tinted(lipgloss.NewStyle(), sel).Render("  ") +
		tinted(style, sel).Render(fit(word, cw-2, false))
}

// snippetLinesFor wraps LastAssistantText to at most snippetLines dim lines.
func snippetLinesFor(s *registry.Session, cw int, sel bool) []string {
	text := s.Stats.LastAssistantText
	if text == "" {
		return nil
	}
	var out []string
	for _, ln := range wrapWords(text, cw-2, snippetLines) {
		out = append(out, tinted(lipgloss.NewStyle(), sel).Render("  ")+
			tinted(blockSnippet, sel).Render(fit(ln, cw-2, false)))
	}
	return out
}

// wrapWords greedily wraps s into at most maxLines lines of width w; the
// last line is ellipsized when text remains.
func wrapWords(s string, w, maxLines int) []string {
	if w < 2 {
		return nil
	}
	words := strings.Fields(s)
	var lines []string
	cur := ""
	for i := 0; i < len(words); i++ {
		wd := words[i]
		next := wd
		if cur != "" {
			next = cur + " " + wd
		}
		if lipgloss.Width(next) <= w {
			cur = next
			continue
		}
		if cur == "" { // single overlong word
			cur = wd
		} else {
			i--
		}
		lines = append(lines, cur)
		cur = ""
		if len(lines) == maxLines {
			last := lines[maxLines-1]
			return append(lines[:maxLines-1], truncate(last+"…", w))
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return lines
}

// detailCard renders the bordered inline expansion for the selected block.
func detailCard(s *registry.Session, cw int) []string {
	inner := cw - 4 // "│ " + " │"
	if inner < 10 {
		inner = 10
	}
	colA := []string{
		cardLabel.Render("context ") + gauge24(s) + " " + cardValue.Render(contextText(s)),
		cardLabel.Render("cost    ") + cardAmber.Render(fmt.Sprintf("$%.2f", s.Stats.CostUSD)) +
			cardDim.Render(" · ") + cardValue.Render(modelEffort(s)),
		cardLabel.Render("where   ") + cardDim.Render(whereText(s)),
		cardLabel.Render("perm    ") + cardValue.Render(orDash(s.Stats.PermissionMode)),
	}
	colB := []string{
		cardLabel.Render("skills  ") + cardAmber.Render(orDash(strings.Join(s.Stats.Skills, ", "))),
		cardLabel.Render("mcp     ") + cardValue.Render(orDash(strings.Join(s.Stats.MCPServers, ", "))),
		cardLabel.Render("agents  ") + agentsText(s),
		cardLabel.Render("turns   ") + cardSpark.Render(sparkline(s.Stats.TokensPerTurn, sparkTurns)) +
			cardDim.Render(" · ") + cardValue.Render(sessionText(s)),
	}

	var rows []string
	if cw >= twoColMinW {
		colW := (inner - 2) / 2
		for i := range colA {
			rows = append(rows, fitStyled(colA[i], colW)+"  "+fitStyled(colB[i], inner-colW-2))
		}
	} else {
		for _, ln := range append(colA, colB...) {
			rows = append(rows, fitStyled(ln, inner))
		}
	}

	h := strings.Repeat("─", cw-2)
	out := []string{cardBorder.Render("╭" + h + "╮")}
	side := cardBorder.Render("│")
	for _, r := range rows {
		out = append(out, side+" "+r+" "+side)
	}
	return append(out, cardBorder.Render("╰"+h+"╯"))
}

// gauge24 renders the 24-cell context gauge with a green→amber→red gradient
// over the filled cells.
func gauge24(s *registry.Session) string {
	frac := contextFrac(s.Stats.ContextTokens, s.Stats.ContextLimit)
	filled := int(frac*detailGaugeW + 0.5)
	if filled > detailGaugeW {
		filled = detailGaugeW
	}
	var b strings.Builder
	for i := 0; i < detailGaugeW; i++ {
		if i >= filled {
			b.WriteString(gaugeEmpty.Render("░"))
			continue
		}
		pos := float64(i) / float64(detailGaugeW)
		switch {
		case pos >= 0.85:
			b.WriteString(gaugeRed.Render("█"))
		case pos >= 0.60:
			b.WriteString(gaugeAmber.Render("█"))
		default:
			b.WriteString(gaugeGreen.Render("█"))
		}
	}
	return b.String()
}

func contextText(s *registry.Session) string {
	frac := contextFrac(s.Stats.ContextTokens, s.Stats.ContextLimit)
	return fmt.Sprintf("%s/%s (%.0f%%)",
		humanTokens(s.Stats.ContextTokens), humanTokens(s.Stats.ContextLimit), frac*100)
}

func modelEffort(s *registry.Session) string {
	m := shortModel(s.Stats.Model)
	if m == "" {
		return "—"
	}
	if s.Stats.Effort != "" {
		m += " " + s.Stats.Effort
	}
	return m
}

func whereText(s *registry.Session) string {
	var parts []string
	if s.Signal.Project != "" {
		parts = append(parts, s.Signal.Project)
	}
	if s.Stats.GitBranch != "" {
		parts = append(parts, s.Stats.GitBranch)
	}
	if s.Signal.Cwd != "" {
		parts = append(parts, s.Signal.Cwd)
	}
	return orDash(strings.Join(parts, " · "))
}

func agentsText(s *registry.Session) string {
	if s.TotalAgents == 0 {
		return cardDim.Render("—")
	}
	out := cardValue.Render(fmt.Sprintf("⚑%d live / %d total", s.LiveAgents, s.TotalAgents))
	if len(s.AgentNames) > 0 {
		out += cardDim.Render(" · " + strings.Join(s.AgentNames, ", "))
	}
	return out
}

func sessionText(s *registry.Session) string {
	dur := "—"
	if !s.Stats.FirstTs.IsZero() && !s.Stats.LastTs.IsZero() {
		dur = humanDur(s.Stats.LastTs.Sub(s.Stats.FirstTs))
	}
	return fmt.Sprintf("%d turns · %s", s.Stats.Turns, dur)
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

// modeWord maps a transcript mode to the block label: Chat unless planning.
func modeWord(mode string) string {
	if strings.EqualFold(mode, "plan") {
		return "Plan"
	}
	return "Chat"
}

var sparkRunes = []rune("▁▂▃▄▅▆▇█")

// sparkline renders the last n values normalized to their max.
func sparkline(vals []int64, n int) string {
	if len(vals) > n {
		vals = vals[len(vals)-n:]
	}
	if len(vals) == 0 {
		return "—"
	}
	var max int64 = 1
	for _, v := range vals {
		if v > max {
			max = v
		}
	}
	var b strings.Builder
	for _, v := range vals {
		i := int(v * int64(len(sparkRunes)-1) / max)
		b.WriteRune(sparkRunes[i])
	}
	return b.String()
}

// humanTokens: 285000 → 285k, 1000000 → 1M.
func humanTokens(v int64) string {
	switch {
	case v >= 1_000_000 && v%1_000_000 == 0:
		return fmt.Sprintf("%dM", v/1_000_000)
	case v >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(v)/1e6)
	case v >= 1_000:
		return fmt.Sprintf("%dk", v/1_000)
	default:
		return fmt.Sprintf("%d", v)
	}
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

// truncate cuts plain text to display width w with a trailing ellipsis.
func truncate(s string, w int) string {
	if lipgloss.Width(s) <= w {
		return s
	}
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
	return b.String() + "…"
}

// fit truncates plain text to display width w (ellipsis when cut) and pads
// to exactly w columns. Style AFTER fitting: it must not see ANSI codes.
func fit(s string, w int, right bool) string {
	if w <= 0 {
		return ""
	}
	s = truncate(s, w)
	pad := strings.Repeat(" ", w-lipgloss.Width(s))
	if right {
		return pad + s
	}
	return s + pad
}

// fitStyled pads or hard-cuts an already-styled string to w display cols.
func fitStyled(s string, w int) string {
	got := lipgloss.Width(s)
	if got > w {
		return truncStyled(s, w)
	}
	return s + strings.Repeat(" ", w-got)
}

// truncStyled cuts a styled string to w display columns, preserving escape
// sequences and terminating with a reset.
func truncStyled(s string, w int) string {
	var b strings.Builder
	cw := 0
	inEsc := false
	for _, r := range s {
		if inEsc {
			b.WriteRune(r)
			if r == 'm' {
				inEsc = false
			}
			continue
		}
		if r == '\x1b' {
			inEsc = true
			b.WriteRune(r)
			continue
		}
		rw := lipgloss.Width(string(r))
		if cw+rw > w-1 {
			break
		}
		b.WriteRune(r)
		cw += rw
	}
	out := b.String() + "\x1b[0m…"
	if pad := w - cw - 1; pad > 0 {
		out += strings.Repeat(" ", pad)
	}
	return out
}

// View renders the full screen: slim header, session blocks, footer.
func (m Model) View() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}
	header := m.headerView()
	footer := footerStyle.Render(" ↑↓ move · ⏎ jump · h view · q quit")

	avail := m.height - 2
	if avail < 1 {
		avail = 1
	}
	var lines []string
	if len(m.rows) == 0 {
		lines = append(lines, emptyStyle.Render("  no sessions — press h to widen the view"))
	} else {
		blocks := make([][]string, len(m.rows))
		for i, s := range m.rows {
			blocks[i] = strings.Split(RenderBlock(s, m.width, i == m.cursor, m.tick), "\n")
		}
		start := m.scroll
		if start > m.cursor {
			start = m.cursor
		}
		// Advance start until the cursor block fits in the viewport.
		for start < m.cursor {
			sum := 0
			for i := start; i <= m.cursor; i++ {
				sum += len(blocks[i]) + 1 // + separator
			}
			if sum-1 <= avail {
				break
			}
			start++
		}
		for i := start; i < len(blocks) && len(lines) < avail; i++ {
			if i > start {
				lines = append(lines, "")
			}
			for _, ln := range blocks[i] {
				if len(lines) == avail {
					break
				}
				lines = append(lines, ln)
			}
		}
	}
	for len(lines) < avail {
		lines = append(lines, "")
	}
	return header + "\n" + strings.Join(lines, "\n") + "\n" + footer
}

// headerView is a slim single line: name, counts, sessions, cost, mode, clock.
func (m Model) headerView() string {
	var working, idle, attn int
	var cost float64
	for _, s := range m.rows {
		cost += s.Stats.CostUSD
		if s.State != registry.StateLive {
			continue
		}
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
	left := " " + appName.Render("claude_top") + "  " +
		stWorking.Render(fmt.Sprintf("⚡%d", working)) + " " +
		stIdle.Render(fmt.Sprintf("💤%d", idle)) + " " +
		stAttention.Render(fmt.Sprintf("🔴%d", attn)) + sep +
		headerMeta.Render(fmt.Sprintf("%d sessions", len(m.rows))) + sep +
		cardAmber.Render(fmt.Sprintf("$%.0f today", cost)) + sep +
		appName.Render("["+modeLabel(m.mode)+"]")
	clock := clockStyle.Render(now().Format("15:04:05")) + " "
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(clock)
	if gap < 1 {
		return truncStyled(left+" "+clock, m.width)
	}
	return left + strings.Repeat(" ", gap) + clock
}

func modeLabel(mode registry.Mode) string {
	switch mode {
	case registry.ModeLive:
		return "live"
	case registry.ModeLiveRecent:
		return "live+recent"
	default:
		return "all"
	}
}
