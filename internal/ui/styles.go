package ui

import "github.com/charmbracelet/lipgloss"

// accent is the single accent color (claude amber).
var accent = lipgloss.Color("#FF8700")

var (
	workingColor   = lipgloss.AdaptiveColor{Light: "28", Dark: "42"}
	idleColor      = lipgloss.AdaptiveColor{Light: "245", Dark: "245"}
	attentionColor = lipgloss.AdaptiveColor{Light: "160", Dark: "196"}

	headerBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(accent).
			Padding(0, 1)
	appName        = lipgloss.NewStyle().Bold(true).Foreground(accent)
	headerSep      = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "250", Dark: "240"})
	modeLabelStyle = lipgloss.NewStyle().Bold(true).Foreground(accent)

	stWorking   = lipgloss.NewStyle().Foreground(workingColor)
	stIdle      = lipgloss.NewStyle().Foreground(idleColor)
	stAttention = lipgloss.NewStyle().Bold(true).Foreground(attentionColor)
	stFlash     = stAttention.Reverse(true)

	rowTitle  = lipgloss.NewStyle().Bold(true)
	rowMeta   = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "240", Dark: "247"})
	rowModel  = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "61", Dark: "104"})
	rowCost   = lipgloss.NewStyle().Foreground(accent)
	rowAgents = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "33", Dark: "39"})
	rowTool   = lipgloss.NewStyle().Italic(true).Foreground(lipgloss.AdaptiveColor{Light: "244", Dark: "244"})

	gaugeGreen = lipgloss.NewStyle().Foreground(workingColor)
	gaugeAmber = lipgloss.NewStyle().Foreground(accent)
	gaugeRed   = lipgloss.NewStyle().Foreground(attentionColor)

	// dimRow for recently-dead sessions, dimmerRow for history.
	dimRow    = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "245", Dark: "243"})
	dimmerRow = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "250", Dark: "238"})

	cursorStyle = lipgloss.NewStyle().Bold(true).Foreground(accent)
	emptyStyle  = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "245", Dark: "243"})
	footerStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "246", Dark: "241"})
)

func gaugeStyleFor(frac float64) lipgloss.Style {
	switch {
	case frac >= 0.85:
		return gaugeRed
	case frac >= 0.60:
		return gaugeAmber
	default:
		return gaugeGreen
	}
}
