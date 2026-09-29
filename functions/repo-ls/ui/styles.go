package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/dustinmichels/repo-ls/models"
)

var (
	// Base colors
	purpleColor  = lipgloss.Color("#7D56F4")
	cyanColor    = lipgloss.Color("#00D7D7")
	greenColor   = lipgloss.Color("#00AF5F")
	redColor     = lipgloss.Color("#FF5F87")
	orangeColor  = lipgloss.Color("#FFAF00")
	yellowColor  = lipgloss.Color("#FFD700")
	grayColor    = lipgloss.Color("#888888")
	darkGrayColor = lipgloss.Color("#444444")
	subtleColor  = lipgloss.Color("#333333")

	// Header
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(purpleColor).
			Padding(0, 1)

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#5A32A3")).
			Padding(0, 1)

	headerPathStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(cyanColor)

	dimStyle = lipgloss.NewStyle().
			Foreground(grayColor)

	subtleStyle = lipgloss.NewStyle().
			Foreground(darkGrayColor)

	accentStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(cyanColor)

	warningStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(orangeColor)

	dangerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(redColor)

	successStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(greenColor)

	// Badges
	badgeClean = lipgloss.NewStyle().
			Bold(true).
			Foreground(greenColor).
			Render("✓ clean")

	badgeDirty = lipgloss.NewStyle().
			Bold(true).
			Foreground(redColor).
			Render("● dirty")

	badgePublic = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(greenColor).
			Padding(0, 1).
			Render("PUBLIC")

	badgePrivate = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#000000")).
			Background(yellowColor).
			Padding(0, 1).
			Render("PRIVATE")

	badgeLocal = lipgloss.NewStyle().
			Foreground(grayColor).
			Render("local")

	badgeUnknown = lipgloss.NewStyle().
			Foreground(darkGrayColor).
			Render("unknown")

	// Panels
	paneLeftStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#555555")).
			Padding(0, 0)

	paneRightStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#7D56F4")).
			Padding(0, 1)

	tableHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(purpleColor).
				Border(lipgloss.NormalBorder(), false, false, true, false).
				BorderForeground(darkGrayColor)

	selectedRowStyle = lipgloss.NewStyle().
				Bold(true).
				Background(lipgloss.Color("#2A2A3D")).
				Foreground(lipgloss.Color("#FFFFFF"))

	modalStyle = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(redColor).
			Padding(1, 2).
			Bold(true)

	searchPromptStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(orangeColor)
)

// RenderVisibilityBadge returns styled string for GitHub visibility.
func RenderVisibilityBadge(vis models.Visibility) string {
	switch vis {
	case models.VisibilityPublic:
		return badgePublic
	case models.VisibilityPrivate:
		return badgePrivate
	case models.VisibilityLocal:
		return badgeLocal
	default:
		return badgeUnknown
	}
}

// RenderStatusBadge returns styled status for clean/dirty state.
func RenderStatusBadge(hasUncommitted bool, count int) string {
	if !hasUncommitted {
		return badgeClean
	}
	if count > 0 {
		return lipgloss.NewStyle().Bold(true).Foreground(redColor).Render("● dirty")
	}
	return badgeDirty
}

// ProgressBar generates a small visual bar for language percentages.
func ProgressBar(width int, fraction float64, color lipgloss.Color) string {
	if width <= 0 {
		return ""
	}
	fillCount := int(fraction * float64(width))
	if fillCount > width {
		fillCount = width
	}
	if fillCount < 0 {
		fillCount = 0
	}
	emptyCount := width - fillCount

	filled := strings.Repeat("█", fillCount)
	empty := strings.Repeat("░", emptyCount)

	return lipgloss.NewStyle().Foreground(color).Render(filled) +
		lipgloss.NewStyle().Foreground(darkGrayColor).Render(empty)
}
