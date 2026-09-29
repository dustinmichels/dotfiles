package main

import (
	"fmt"
	"math"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ViewMode int

const (
	ViewHorizontal ViewMode = iota
	ViewVertical
)

type DiffMode int

const (
	DiffPrev DiffMode = iota
	DiffLatest
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1)

	subTitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#AAAAAA")).
			Padding(0, 1)

	activeTabStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#000000")).
			Background(lipgloss.Color("#00D7D7")).
			Padding(0, 1)

	inactiveTabStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")).
			Background(lipgloss.Color("#222222")).
			Padding(0, 1)

	cardStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#444444")).
			Padding(0, 1)

	accentStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#00D7D7"))

	successStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#00AF5F"))

	dangerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FF5F87"))

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666"))

	selectedRowStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#00D7D7"))
)

type reloadMsg struct {
	snapshots []*CommitSnapshot
	err       error
}

type Model struct {
	repoRoot      string
	snapshots     []*CommitSnapshot
	selectedIndex int
	activeMetric  Metric
	viewMode      ViewMode
	diffMode      DiffMode
	scrollOffset  int
	width         int
	height        int
	showHelp      bool
	limit         int
	loading       bool
	statusMsg     string
	err           error
}

func NewModel(repoRoot string, initialSnapshots []*CommitSnapshot, limit int) Model {
	idx := len(initialSnapshots) - 1
	if idx < 0 {
		idx = 0
	}
	return Model{
		repoRoot:      repoRoot,
		snapshots:     initialSnapshots,
		selectedIndex: idx, // start with the latest commit selected
		activeMetric:  MetricCode,
		viewMode:      ViewHorizontal,
		diffMode:      DiffPrev,
		limit:         limit,
		width:         100,
		height:        30,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ensureSelectionVisible()
		return m, nil

	case reloadMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			m.statusMsg = fmt.Sprintf("Error: %v", msg.err)
		} else {
			m.snapshots = msg.snapshots
			if m.selectedIndex >= len(m.snapshots) {
				m.selectedIndex = len(m.snapshots) - 1
			}
			m.statusMsg = fmt.Sprintf("Loaded %d commits", len(m.snapshots))
			m.ensureSelectionVisible()
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "?":
			m.showHelp = !m.showHelp
			return m, nil

		// Metric cycling (Tab / Shift+Tab)
		case "tab":
			m.activeMetric = NextMetric(m.activeMetric)
			return m, nil

		case "shift+tab":
			m.activeMetric = PrevMetric(m.activeMetric)
			return m, nil

		// Direct metric shortcuts
		case "1":
			m.activeMetric = MetricCode
			return m, nil
		case "2":
			m.activeMetric = MetricLines
			return m, nil
		case "3":
			m.activeMetric = MetricFiles
			return m, nil
		case "4":
			m.activeMetric = MetricComments
			return m, nil
		case "5":
			m.activeMetric = MetricBlanks
			return m, nil

		// Navigation
		case "up", "k":
			if m.selectedIndex > 0 {
				m.selectedIndex--
				m.ensureSelectionVisible()
			}
			return m, nil

		case "down", "j":
			if m.selectedIndex < len(m.snapshots)-1 {
				m.selectedIndex++
				m.ensureSelectionVisible()
			}
			return m, nil

		case "left", "h":
			if m.viewMode == ViewVertical && m.selectedIndex > 0 {
				m.selectedIndex--
			} else if m.selectedIndex > 0 {
				m.selectedIndex--
				m.ensureSelectionVisible()
			}
			return m, nil

		case "right", "l":
			if m.viewMode == ViewVertical && m.selectedIndex < len(m.snapshots)-1 {
				m.selectedIndex++
			} else if m.selectedIndex < len(m.snapshots)-1 {
				m.selectedIndex++
				m.ensureSelectionVisible()
			}
			return m, nil

		case "home", "g":
			m.selectedIndex = 0
			m.ensureSelectionVisible()
			return m, nil

		case "end", "G":
			m.selectedIndex = len(m.snapshots) - 1
			m.ensureSelectionVisible()
			return m, nil

		// View Mode toggle
		case "v":
			if m.viewMode == ViewHorizontal {
				m.viewMode = ViewVertical
			} else {
				m.viewMode = ViewHorizontal
			}
			return m, nil

		// Diff Mode toggle
		case "d", "c":
			if m.diffMode == DiffPrev {
				m.diffMode = DiffLatest
			} else {
				m.diffMode = DiffPrev
			}
			return m, nil

		// Increase/decrease commits
		case "+", "=":
			m.limit += 5
			m.loading = true
			m.statusMsg = fmt.Sprintf("Fetching %d commits...", m.limit)
			return m, m.reloadCommitsCmd()

		case "-":
			if m.limit > 5 {
				m.limit -= 5
				m.loading = true
				m.statusMsg = fmt.Sprintf("Fetching %d commits...", m.limit)
				return m, m.reloadCommitsCmd()
			}
			return m, nil

		// Refresh
		case "r":
			m.loading = true
			m.statusMsg = "Refreshing data..."
			return m, m.reloadCommitsCmd()
		}
	}

	return m, nil
}

func (m *Model) ensureSelectionVisible() {
	if len(m.snapshots) == 0 {
		return
	}
	visibleRows := m.maxVisibleListRows()
	if visibleRows <= 0 {
		visibleRows = 6
	}

	if m.selectedIndex < m.scrollOffset {
		m.scrollOffset = m.selectedIndex
	}
	if m.selectedIndex >= m.scrollOffset+visibleRows {
		m.scrollOffset = m.selectedIndex - visibleRows + 1
	}
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
}

func (m Model) maxVisibleListRows() int {
	// Total height minus Header(4), Card(10), Legend(2), Footer(2)
	avail := m.height - 18
	if avail < 4 {
		return 4
	}
	if avail > 16 {
		return 16
	}
	return avail
}

func (m Model) reloadCommitsCmd() tea.Cmd {
	return func() tea.Msg {
		snapshots, err := CollectHistory(m.repoRoot, m.limit, nil)
		return reloadMsg{snapshots: snapshots, err: err}
	}
}

func (m Model) View() string {
	if m.width == 0 {
		return "Loading tokei-time..."
	}

	var sb strings.Builder

	// 1. Header with title and metric tabs
	sb.WriteString(m.renderHeader())
	sb.WriteString("\n\n")

	// 2. Bar Graph (Horizontal or Vertical)
	if len(m.snapshots) == 0 {
		sb.WriteString(dimStyle.Render("  No commits found."))
		sb.WriteString("\n\n")
	} else if m.viewMode == ViewHorizontal {
		sb.WriteString(m.renderHorizontalView())
		sb.WriteString("\n")
	} else {
		sb.WriteString(m.renderVerticalView())
		sb.WriteString("\n")
	}

	// 3. Detail Pane for Selected Commit
	if len(m.snapshots) > 0 && m.selectedIndex >= 0 && m.selectedIndex < len(m.snapshots) {
		sb.WriteString(m.renderDetailCard())
		sb.WriteString("\n")
	}

	// 4. Language Legend
	sb.WriteString(m.renderLegend())
	sb.WriteString("\n")

	// 5. Footer & Keybindings
	sb.WriteString(m.renderFooter())

	return sb.String()
}

func (m Model) renderHeader() string {
	logo := titleStyle.Render("TOKEI-TIME")
	repoName := strings.TrimPrefix(m.repoRoot, "/Users/")
	if len(repoName) > 30 {
		repoName = "..." + repoName[len(repoName)-27:]
	}
	sub := subTitleStyle.Render(fmt.Sprintf("%s (%d commits)", repoName, len(m.snapshots)))

	var tabs []string
	for _, metric := range MetricList {
		name := MetricNames[metric]
		if metric == m.activeMetric {
			tabs = append(tabs, activeTabStyle.Render(fmt.Sprintf(" %s ", name)))
		} else {
			tabs = append(tabs, inactiveTabStyle.Render(fmt.Sprintf(" %s ", name)))
		}
	}
	tabBar := strings.Join(tabs, " ")

	return lipgloss.JoinHorizontal(lipgloss.Center, logo, sub, "   ", tabBar)
}

func (m Model) renderHorizontalView() string {
	var sb strings.Builder

	maxValue := 0
	for _, snap := range m.snapshots {
		v := snap.Total.Value(m.activeMetric)
		if v > maxValue {
			maxValue = v
		}
	}

	visibleRows := m.maxVisibleListRows()
	start := m.scrollOffset
	end := start + visibleRows
	if end > len(m.snapshots) {
		end = len(m.snapshots)
	}

	// Calculate bar width based on terminal width
	// Left side columns: Cursor(2) + Badge(5) + Hash(8) + Time(9) + Subject(20) = 44
	// Right side columns: Value(12) + Delta(8) = 20
	// Bar width = width - 68
	barWidth := m.width - 68
	if barWidth < 15 {
		barWidth = 15
	}
	if barWidth > 60 {
		barWidth = 60
	}

	diffLabel := "Δ Prev"
	if m.diffMode == DiffLatest {
		diffLabel = "Δ Latest"
	}
	headerRow := fmt.Sprintf("   %-5s %-8s %-9s %-20s %-*s %11s  %-8s",
		"TYPE", "COMMIT", "WHEN", "SUBJECT", barWidth, "BREAKDOWN", strings.ToUpper(MetricNames[m.activeMetric]), diffLabel)
	sb.WriteString(dimStyle.Render(headerRow))
	sb.WriteString("\n")

	for i := start; i < end; i++ {
		snap := m.snapshots[i]
		isSelected := i == m.selectedIndex

		cursor := "  "
		if isSelected {
			cursor = accentStyle.Render("▶ ")
		}

		badge := "     "
		if snap.IsWorkingTree {
			badge = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFAF00")).Bold(true).Render("[WT] ")
		} else if i == len(m.snapshots)-1 || (i == len(m.snapshots)-2 && m.snapshots[len(m.snapshots)-1].IsWorkingTree) {
			badge = lipgloss.NewStyle().Foreground(lipgloss.Color("#00D7D7")).Render("[HD] ")
		}

		hash := snap.ShortHash
		if len(hash) > 7 {
			hash = hash[:7]
		}
		hashStyled := dimStyle.Render(fmt.Sprintf("%-7s", hash))
		if isSelected {
			hashStyled = selectedRowStyle.Render(fmt.Sprintf("%-7s", hash))
		}

		timeStr := snap.RelativeDate
		if len(timeStr) > 8 {
			timeStr = timeStr[:8]
		}
		timeStyled := dimStyle.Render(fmt.Sprintf("%-8s", timeStr))

		subj := snap.Subject
		if len(subj) > 19 {
			subj = subj[:18] + "…"
		}
		subjStyled := fmt.Sprintf("%-19s", subj)
		if isSelected {
			subjStyled = lipgloss.NewStyle().Bold(true).Render(subjStyled)
		} else {
			subjStyled = dimStyle.Render(subjStyled)
		}

		// Calculate stacked bar
		totalVal, bWidth, segments := CalculateHorizontalSegments(snap, m.activeMetric, barWidth, maxValue)

		var barBuf strings.Builder
		for _, seg := range segments {
			style := lipgloss.NewStyle().Foreground(lipgloss.Color(seg.Color))
			barBuf.WriteString(style.Render(strings.Repeat("█", seg.Chars)))
		}
		if bWidth < barWidth {
			barBuf.WriteString(dimStyle.Render(strings.Repeat("·", barWidth-bWidth)))
		}

		// Metric Value
		valStr := formatNumber(totalVal)
		valStyled := fmt.Sprintf("%11s", valStr)
		if isSelected {
			valStyled = accentStyle.Render(valStyled)
		}

		// Delta
		var delta int
		if m.diffMode == DiffLatest {
			delta = snap.TotalDiffLatest.Value(m.activeMetric)
		} else {
			delta = snap.TotalDiffPrev.Value(m.activeMetric)
		}

		var deltaStyled string
		if i == 0 && m.diffMode == DiffPrev {
			deltaStyled = dimStyle.Render("       -")
		} else if delta > 0 {
			deltaStyled = successStyle.Render(fmt.Sprintf("%+8d", delta))
		} else if delta < 0 {
			deltaStyled = dangerStyle.Render(fmt.Sprintf("%+8d", delta))
		} else {
			deltaStyled = dimStyle.Render("       0")
		}

		row := fmt.Sprintf("%s%s%s %s %s %s %s %s\n",
			cursor, badge, hashStyled, timeStyled, subjStyled, barBuf.String(), valStyled, deltaStyled)
		sb.WriteString(row)
	}

	return sb.String()
}

func (m Model) renderVerticalView() string {
	var sb strings.Builder

	chartHeight := 10
	maxValue := 0
	for _, snap := range m.snapshots {
		v := snap.Total.Value(m.activeMetric)
		if v > maxValue {
			maxValue = v
		}
	}

	cols := make([][]string, len(m.snapshots))
	for i, snap := range m.snapshots {
		cols[i] = CalculateVerticalColumn(snap, m.activeMetric, chartHeight, maxValue)
	}

	// Render rows from top (chartHeight-1) down to 0
	for r := chartHeight - 1; r >= 0; r-- {
		// Y-axis label
		valAtRow := int(float64(maxValue) * float64(r+1) / float64(chartHeight))
		axisLabel := fmt.Sprintf("%7s ┤ ", formatCompactNumber(valAtRow))
		sb.WriteString(dimStyle.Render(axisLabel))

		for c, col := range cols {
			color := col[r]
			if color == "" {
				sb.WriteString("    ")
			} else {
				style := lipgloss.NewStyle().Foreground(lipgloss.Color(color))
				if c == m.selectedIndex {
					sb.WriteString(style.Bold(true).Render("███ "))
				} else {
					sb.WriteString(style.Render("███ "))
				}
			}
		}
		sb.WriteString("\n")
	}

	// X-axis border
	sb.WriteString(dimStyle.Render("        ┴"))
	for range len(m.snapshots) {
		sb.WriteString(dimStyle.Render("────"))
	}
	sb.WriteString("\n")

	// Selector marker row
	sb.WriteString("         ")
	for c := range len(m.snapshots) {
		if c == m.selectedIndex {
			sb.WriteString(accentStyle.Render(" ▲  "))
		} else {
			sb.WriteString("    ")
		}
	}
	sb.WriteString("\n")

	// Commit labels row
	sb.WriteString("         ")
	for c, snap := range m.snapshots {
		label := snap.ShortHash
		if snap.IsWorkingTree {
			label = " WT"
		} else if len(label) > 3 {
			label = label[:3]
		}
		label = fmt.Sprintf("%-3s ", label)
		if c == m.selectedIndex {
			sb.WriteString(selectedRowStyle.Render(label))
		} else {
			sb.WriteString(dimStyle.Render(label))
		}
	}
	sb.WriteString("\n")

	return sb.String()
}

func (m Model) renderDetailCard() string {
	snap := m.snapshots[m.selectedIndex]

	var lines []string

	// Header line
	title := fmt.Sprintf("Commit: %s", snap.Hash)
	if snap.IsWorkingTree {
		title = "Working Tree (Uncommitted Changes)"
	}
	lines = append(lines, accentStyle.Render(title))

	meta := fmt.Sprintf("Author: %s • %s (%s)", snap.Author, snap.RelativeDate, snap.Date)
	if snap.Subject != "" {
		meta += fmt.Sprintf(" • Subject: %s", snap.Subject)
	}
	lines = append(lines, dimStyle.Render(meta))

	// Metric stats & diffs
	metricName := MetricNames[m.activeMetric]
	totalVal := snap.Total.Value(m.activeMetric)

	deltaPrev := snap.TotalDiffPrev.Value(m.activeMetric)
	deltaLatest := snap.TotalDiffLatest.Value(m.activeMetric)

	diffPrevStr := FormatDelta(deltaPrev)
	if deltaPrev > 0 {
		diffPrevStr = successStyle.Render(diffPrevStr)
	} else if deltaPrev < 0 {
		diffPrevStr = dangerStyle.Render(diffPrevStr)
	} else {
		diffPrevStr = dimStyle.Render("0")
	}

	diffLatestStr := FormatDelta(deltaLatest)
	if deltaLatest > 0 {
		diffLatestStr = successStyle.Render(diffLatestStr)
	} else if deltaLatest < 0 {
		diffLatestStr = dangerStyle.Render(diffLatestStr)
	} else {
		diffLatestStr = dimStyle.Render("0")
	}

	statLine := fmt.Sprintf("Total %s: %s   Δ vs Previous: %s   Δ vs Latest: %s",
		metricName, accentStyle.Render(formatNumber(totalVal)), diffPrevStr, diffLatestStr)

	if snap.LinesAdded > 0 || snap.LinesDeleted > 0 {
		statLine += fmt.Sprintf("   git: %s / %s lines",
			successStyle.Render(fmt.Sprintf("+%d", snap.LinesAdded)),
			dangerStyle.Render(fmt.Sprintf("-%d", snap.LinesDeleted)))
	}
	lines = append(lines, statLine)

	// Language breakdown table
	type langRow struct {
		name        string
		val         int
		pct         float64
		deltaPrev   int
		deltaLatest int
		files       int
		lines       int
	}

	var rows []langRow
	for name, stats := range snap.Languages {
		v := stats.Value(m.activeMetric)
		pct := 0.0
		if totalVal > 0 {
			pct = float64(v) / float64(totalVal) * 100.0
		}
		dp := snap.DiffPrev[name].Value(m.activeMetric)
		dl := snap.DiffLatest[name].Value(m.activeMetric)

		rows = append(rows, langRow{
			name:        name,
			val:         v,
			pct:         pct,
			deltaPrev:   dp,
			deltaLatest: dl,
			files:       stats.Files,
			lines:       stats.Lines,
		})
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].val == rows[j].val {
			return rows[i].name < rows[j].name
		}
		return rows[i].val > rows[j].val
	})

	// Show top 6 languages
	var tableLines []string
	tableHeader := fmt.Sprintf("  %-16s %10s %8s %10s %10s %8s %9s",
		"LANGUAGE", metricName, "SHARE", "Δ PREV", "Δ LATEST", "FILES", "LINES")
	tableLines = append(tableLines, dimStyle.Render(tableHeader))

	limitRows := len(rows)
	if limitRows > 6 {
		limitRows = 6
	}

	for i := range limitRows {
		r := rows[i]
		color := GetLanguageColor(r.name)
		icon := lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render("■")

		rawDp := FormatDelta(r.deltaPrev)
		var dpStr string
		if r.deltaPrev > 0 {
			dpStr = successStyle.Render(fmt.Sprintf("%10s", rawDp))
		} else if r.deltaPrev < 0 {
			dpStr = dangerStyle.Render(fmt.Sprintf("%10s", rawDp))
		} else {
			dpStr = dimStyle.Render(fmt.Sprintf("%10s", rawDp))
		}

		rawDl := FormatDelta(r.deltaLatest)
		var dlStr string
		if r.deltaLatest > 0 {
			dlStr = successStyle.Render(fmt.Sprintf("%10s", rawDl))
		} else if r.deltaLatest < 0 {
			dlStr = dangerStyle.Render(fmt.Sprintf("%10s", rawDl))
		} else {
			dlStr = dimStyle.Render(fmt.Sprintf("%10s", rawDl))
		}

		line := fmt.Sprintf("  %s %-14s %10s %7.1f%% %s %s %8d %9s",
			icon, r.name, formatNumber(r.val), r.pct, dpStr, dlStr, r.files, formatNumber(r.lines))
		tableLines = append(tableLines, line)
	}

	if len(rows) > limitRows {
		tableLines = append(tableLines, dimStyle.Render(fmt.Sprintf("  ... and %d more languages", len(rows)-limitRows)))
	}

	lines = append(lines, strings.Join(tableLines, "\n"))

	cardWidth := m.width - 4
	if cardWidth < 60 {
		cardWidth = 60
	}
	return cardStyle.Width(cardWidth).Render(strings.Join(lines, "\n"))
}

func (m Model) renderLegend() string {
	// Collect languages present across all commits
	langSet := make(map[string]bool)
	for _, snap := range m.snapshots {
		for l, s := range snap.Languages {
			if s.Value(m.activeMetric) > 0 {
				langSet[l] = true
			}
		}
	}

	var langs []string
	for l := range langSet {
		langs = append(langs, l)
	}
	sort.Strings(langs)

	if len(langs) > 10 {
		langs = langs[:10]
	}

	var items []string
	for _, l := range langs {
		color := GetLanguageColor(l)
		swatch := lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render("■")
		items = append(items, fmt.Sprintf("%s %s", swatch, l))
	}

	legend := strings.Join(items, "  ")
	return dimStyle.Render("Languages: ") + legend
}

func (m Model) renderFooter() string {
	keys := []string{
		accentStyle.Render("Tab") + ": Metric",
		accentStyle.Render("1-5") + ": Jump",
		accentStyle.Render("↑/↓") + " or " + accentStyle.Render("j/k") + ": Nav",
		accentStyle.Render("v") + ": Toggle View",
		accentStyle.Render("c") + ": Diff Mode",
		accentStyle.Render("+/-") + ": Commits",
		accentStyle.Render("q") + ": Quit",
	}

	bar := strings.Join(keys, " • ")
	if m.statusMsg != "" {
		return bar + "  " + dimStyle.Render(m.statusMsg)
	}
	return bar
}

func formatNumber(n int) string {
	if n < 0 {
		return "-" + formatNumber(-n)
	}
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}

	var res []string
	rem := len(s) % 3
	if rem > 0 {
		res = append(res, s[:rem])
	}
	for i := rem; i < len(s); i += 3 {
		res = append(res, s[i:i+3])
	}
	return strings.Join(res, ",")
}

func formatCompactNumber(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	} else if n < 1000000 {
		val := float64(n) / 1000.0
		if val < 10.0 {
			return fmt.Sprintf("%.1fk", val)
		}
		return fmt.Sprintf("%dk", int(math.Round(val)))
	}
	val := float64(n) / 1000000.0
	return fmt.Sprintf("%.1fM", val)
}
