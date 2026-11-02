package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	accentColor = lipgloss.Color("#A78BFA")
	mutedColor  = lipgloss.Color("#6B7280")
	greenColor  = lipgloss.Color("#10B981")
	redColor    = lipgloss.Color("#EF4444")
	amberColor  = lipgloss.Color("#F59E0B")

	titleStyle   = lipgloss.NewStyle().Bold(true)
	sectionStyle = lipgloss.NewStyle().Bold(true).Foreground(accentColor)
	nameStyle    = lipgloss.NewStyle().Bold(true)
	dimStyle     = lipgloss.NewStyle().Foreground(mutedColor)
	okStyle      = lipgloss.NewStyle().Foreground(greenColor)
	updStyle     = lipgloss.NewStyle().Bold(true).Foreground(greenColor)
	errStyle     = lipgloss.NewStyle().Foreground(redColor)
	warnStyle    = lipgloss.NewStyle().Foreground(amberColor)
	badgeStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#000000")).Background(amberColor).Padding(0, 1)
)

// status is ordered by precedence: a row only ever moves to a higher status.
type status int

const (
	stInfo    status = iota // inventory only; nothing for this tool to update
	stCurrent               // checked/updated, nothing changed
	stPlanned               // dry run: commands would run
	stUpdated               // something changed
	stWarn                  // needs attention
	stFailed                // a command failed
)

func (s status) glyph() string {
	switch s {
	case stCurrent:
		return okStyle.Render("✓")
	case stPlanned:
		return warnStyle.Render("○")
	case stUpdated:
		return updStyle.Render("↑")
	case stWarn:
		return warnStyle.Render("!")
	case stFailed:
		return errStyle.Render("✖")
	default:
		return dimStyle.Render("·")
	}
}

// row is one inventory line: what it is, which version, how it is installed, and what happened.
type row struct {
	st      status
	name    string
	version string
	via     string
	note    string
	detail  []string // indented lines under the row: planned commands or failure output
}

func (r *row) raise(s status) {
	if s > r.st {
		r.st = s
	}
}

func (r *row) addNote(s string) {
	if s == "" {
		return
	}
	if r.note == "" {
		r.note = s
		return
	}
	r.note += " · " + s
}

func versionChange(before, after string) string {
	if before == after || after == "" {
		return before
	}
	if before == "" {
		return after
	}
	return before + " → " + after
}

func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

func pad(s string, w int) string {
	if gap := w - lipgloss.Width(s); gap > 0 {
		return s + strings.Repeat(" ", gap)
	}
	return s
}

func wrap(s string, w int) []string {
	if w < 20 || lipgloss.Width(s) <= w {
		return []string{s}
	}
	var lines []string
	cur := ""
	for _, word := range strings.Fields(s) {
		switch {
		case cur == "":
			cur = word
		case lipgloss.Width(cur)+1+lipgloss.Width(word) > w:
			lines = append(lines, cur)
			cur = word
		default:
			cur += " " + word
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return lines
}

// section prints a titled, column-aligned block of rows and tallies their statuses.
func (a *app) section(title, sub string, rows []row) {
	a.clearProgress()
	if len(rows) == 0 {
		return
	}
	fmt.Printf("\n%s", sectionStyle.Render(title))
	if sub != "" {
		fmt.Printf("  %s", dimStyle.Render(sub))
	}
	fmt.Println()

	var nameW, verW, viaW int
	for _, r := range rows {
		nameW = max(nameW, lipgloss.Width(r.name))
		verW = max(verW, lipgloss.Width(r.version))
		viaW = max(viaW, lipgloss.Width(r.via))
	}

	for _, r := range rows {
		a.counts[r.st]++
		var b strings.Builder
		fmt.Fprintf(&b, "  %s %s", r.st.glyph(), pad(nameStyle.Render(r.name), nameW))
		col := 4 + nameW
		if verW > 0 {
			fmt.Fprintf(&b, "  %s", pad(r.version, verW))
			col += 2 + verW
		}
		if viaW > 0 {
			fmt.Fprintf(&b, "  %s", pad(dimStyle.Render(r.via), viaW))
			col += 2 + viaW
		}
		col += 2

		noteStyle := dimStyle
		switch r.st {
		case stFailed:
			noteStyle = errStyle
		case stWarn:
			noteStyle = warnStyle
		case stUpdated:
			noteStyle = lipgloss.NewStyle()
		}
		for i, line := range wrap(r.note, a.width-col) {
			if line == "" {
				continue
			}
			if i == 0 {
				fmt.Fprintf(&b, "  %s", noteStyle.Render(line))
			} else {
				fmt.Fprintf(&b, "\n%s%s", strings.Repeat(" ", col), noteStyle.Render(line))
			}
		}
		fmt.Println(strings.TrimRight(b.String(), " "))

		detailStyle := dimStyle
		if r.st == stFailed {
			detailStyle = errStyle
		}
		for _, d := range r.detail {
			fmt.Printf("      %s\n", detailStyle.Render(d))
		}
	}
}
