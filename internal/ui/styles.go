package ui

import "github.com/charmbracelet/lipgloss"

// accent is the single accent color (claude amber).
var accent = lipgloss.Color("#FF8700")

var (
	workingColor   = lipgloss.AdaptiveColor{Light: "166", Dark: "214"} // amber family
	idleColor      = lipgloss.AdaptiveColor{Light: "28", Dark: "42"}   // green
	attentionColor = lipgloss.AdaptiveColor{Light: "160", Dark: "196"}
	dimColor       = lipgloss.AdaptiveColor{Light: "245", Dark: "243"}
	dimmerColor    = lipgloss.AdaptiveColor{Light: "250", Dark: "238"}
	metaColor      = lipgloss.AdaptiveColor{Light: "240", Dark: "247"}

	// selBg is the faint background tint of the selected block.
	selBg = lipgloss.AdaptiveColor{Light: "255", Dark: "236"}

	appName    = lipgloss.NewStyle().Bold(true).Foreground(accent)
	headerSep  = lipgloss.NewStyle().Foreground(dimColor)
	headerMeta = lipgloss.NewStyle().Foreground(metaColor)
	clockStyle = lipgloss.NewStyle().Foreground(dimColor)

	stWorking   = lipgloss.NewStyle().Foreground(workingColor)
	stIdle      = lipgloss.NewStyle().Foreground(idleColor)
	stAttention = lipgloss.NewStyle().Bold(true).Foreground(attentionColor)
	stDim       = lipgloss.NewStyle().Foreground(dimColor)
	stDimmer    = lipgloss.NewStyle().Foreground(dimmerColor)

	blockTitle   = lipgloss.NewStyle().Bold(true)
	blockMode    = lipgloss.NewStyle().Foreground(metaColor)
	blockTab     = lipgloss.NewStyle().Foreground(dimColor)
	blockSnippet = lipgloss.NewStyle().Foreground(dimColor)

	// selected-block accent bar and background carrier.
	selBar   = lipgloss.NewStyle().Foreground(accent).Background(selBg)
	selPlain = lipgloss.NewStyle().Background(selBg)

	cardBorder = lipgloss.NewStyle().Foreground(dimmerColor)
	cardLabel  = lipgloss.NewStyle().Foreground(metaColor)
	cardValue  = lipgloss.NewStyle()
	cardDim    = lipgloss.NewStyle().Foreground(dimColor)
	cardAmber  = lipgloss.NewStyle().Foreground(accent)
	cardSpark  = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "61", Dark: "104"})

	gaugeGreen = lipgloss.NewStyle().Foreground(idleColor)
	gaugeAmber = lipgloss.NewStyle().Foreground(accent)
	gaugeRed   = lipgloss.NewStyle().Foreground(attentionColor)
	gaugeEmpty = lipgloss.NewStyle().Foreground(dimmerColor)

	emptyStyle  = lipgloss.NewStyle().Foreground(dimColor)
	footerStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "246", Dark: "241"})
)

// tinted adds the selected-block background tint to base when sel is true.
func tinted(base lipgloss.Style, sel bool) lipgloss.Style {
	if sel {
		return base.Background(selBg)
	}
	return base
}
