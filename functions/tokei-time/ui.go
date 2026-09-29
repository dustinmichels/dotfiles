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

	activeGroupTabStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#000000")).
			Background(lipgloss.Color("#FFAF00")).
			Padding(0, 1)

	inactiveGroupTabStyle = lipgloss.NewStyle().
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

type chunkLoadedMsg struct {
	snapshots        []*CommitSnapshot
	remainingPending []CommitInfo
	err              error
}

type reloadMsg struct {
	snapshots    []*CommitSnapshot
	pendingInfos []CommitInfo
	totalCommits int
	err          error
}

type Model struct {
	repoRoot      string
	rawSnapshots  []*CommitSnapshot
	snapshots     []*CommitSnapshot
	pendingInfos  []CommitInfo
	totalCommits  int
	loadedCommits int
	isLazyLoading bool
	selectedIndex int
	activeMetric  Metric
	activeGroup   GroupMode
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

func NewModel(repoRoot string, initialSnapshots []*CommitSnapshot, limit int, initialGroup GroupMode) Model {
	return NewModelWithLazyLoading(repoRoot, &HistoryInitResult{
		InitialSnapshots: initialSnapshots,
		PendingInfos:     nil,
		TotalCommits:     len(initialSnapshots),
	}, limit, initialGroup)
}

func NewModelWithLazyLoading(repoRoot string, initResult *HistoryInitResult, limit int, initialGroup GroupMode) Model {
	if initResult == nil {
		initResult = &HistoryInitResult{}
	}
	m := Model{
		repoRoot:      repoRoot,
		rawSnapshots:  initResult.InitialSnapshots,
		pendingInfos:  initResult.PendingInfos,
		totalCommits:  initResult.TotalCommits,
		loadedCommits: len(initResult.InitialSnapshots),
		isLazyLoading: len(initResult.PendingInfos) > 0,
		activeMetric:  MetricCode,
		activeGroup:   initialGroup,
		viewMode:      ViewHorizontal,
		diffMode:      DiffPrev,
		limit:         limit,
		width:         100,
		height:        30,
	}
	if m.isLazyLoading && m.totalCommits > 0 {
		pct := float64(m.loadedCommits) / float64(m.totalCommits) * 100
		m.statusMsg = fmt.Sprintf("Streaming history: %d/%d commits (%.0f%%)...", m.loadedCommits, m.totalCommits, pct)
	}
	m.rebuildDisplay()
	m.selectedIndex = len(m.snapshots) - 1
	if m.selectedIndex < 0 {
		m.selectedIndex = 0
	}
	m.ensureSelectionVisible()
	return m
}

func (m *Model) rebuildDisplay() {
	if m.activeGroup == GroupCommit {
		m.snapshots = m.rawSnapshots
	} else {
		m.snapshots = AggregateSnapshots(m.rawSnapshots, m.activeGroup)
	}
	if m.selectedIndex >= len(m.snapshots) {
		m.selectedIndex = len(m.snapshots) - 1
	}
	if m.selectedIndex < 0 {
		m.selectedIndex = 0
	}
	m.ensureSelectionVisible()
}

func (m Model) Init() tea.Cmd {
	if m.isLazyLoading && len(m.pendingInfos) > 0 {
		return m.loadNextChunkCmd()
	}
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ensureSelectionVisible()
		return m, nil

	case chunkLoadedMsg:
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("Error streaming history: %v", msg.err)
			m.isLazyLoading = false
			return m, nil
		}

		m.pendingInfos = msg.remainingPending

		// Remember the currently selected commit's hash to preserve cursor position
		var curHash string
		if m.selectedIndex >= 0 && m.selectedIndex < len(m.snapshots) {
			curHash = m.snapshots[m.selectedIndex].Hash
		}

		// Separate working tree from committed snapshots
		var wtSnap *CommitSnapshot
		var existingCommits []*CommitSnapshot
		for _, s := range m.rawSnapshots {
			if s.IsWorkingTree {
				wtSnap = s
			} else {
				existingCommits = append(existingCommits, s)
			}
		}

		// msg.snapshots are older than existingCommits (we chunk backward chronologically)
		var merged []*CommitSnapshot
		merged = append(merged, msg.snapshots...)
		merged = append(merged, existingCommits...)
		if wtSnap != nil {
			merged = append(merged, wtSnap)
		}
		m.rawSnapshots = merged

		// Recompute diffs across all snapshots
		ComputeDiffs(m.rawSnapshots)

		// Rebuild display for active group
		m.rebuildDisplay()

		// Restore selection to the same commit
		if curHash != "" {
			for i, s := range m.snapshots {
				if s.Hash == curHash {
					m.selectedIndex = i
					break
				}
			}
		}
		m.ensureSelectionVisible()

		m.loadedCommits += len(msg.snapshots)
		if len(m.pendingInfos) > 0 {
			pct := float64(m.loadedCommits) / float64(m.totalCommits) * 100
			m.statusMsg = fmt.Sprintf("Streaming history: %d/%d commits (%.0f%%)...", m.loadedCommits, m.totalCommits, pct)
			return m, m.loadNextChunkCmd()
		}
		m.isLazyLoading = false
		m.statusMsg = fmt.Sprintf("All %d commits loaded", m.totalCommits)
		return m, nil

	case reloadMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			m.statusMsg = fmt.Sprintf("Error: %v", msg.err)
		} else {
			m.rawSnapshots = msg.snapshots
			m.pendingInfos = msg.pendingInfos
			m.totalCommits = msg.totalCommits
			m.loadedCommits = len(msg.snapshots)
			m.isLazyLoading = len(msg.pendingInfos) > 0
			m.rebuildDisplay()
			m.selectedIndex = len(m.snapshots) - 1
			if m.selectedIndex < 0 {
				m.selectedIndex = 0
			}
			m.ensureSelectionVisible()
			if m.isLazyLoading && m.totalCommits > 0 {
				pct := float64(m.loadedCommits) / float64(m.totalCommits) * 100
				m.statusMsg = fmt.Sprintf("Streaming history: %d/%d commits (%.0f%%)...", m.loadedCommits, m.totalCommits, pct)
				return m, m.loadNextChunkCmd()
			}
			m.statusMsg = fmt.Sprintf("Loaded %d commits", len(m.rawSnapshots))
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

		case "home":
			m.selectedIndex = 0
			m.ensureSelectionVisible()
			return m, nil

		case "end":
			m.selectedIndex = len(m.snapshots) - 1
			m.ensureSelectionVisible()
			return m, nil

		// Summary / Group mode cycling
		case "s", "g":
			m.activeGroup = NextGroupMode(m.activeGroup)
			m.rebuildDisplay()
			m.statusMsg = fmt.Sprintf("Group mode: %s", GroupModeNames[m.activeGroup])
			return m, nil

		case "S", "G":
			m.activeGroup = PrevGroupMode(m.activeGroup)
			m.rebuildDisplay()
			m.statusMsg = fmt.Sprintf("Group mode: %s", GroupModeNames[m.activeGroup])
			return m, nil

		// Load all commits
		case "a":
			if m.limit != 0 {
				m.limit = 0
				m.loading = true
				m.statusMsg = "Fetching all commits from history..."
				return m, m.reloadCommitsCmd()
			}
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
			if m.limit == 0 {
				m.statusMsg = "Already displaying all commits"
				return m, nil
			}
			m.limit += 10
			m.loading = true
			m.statusMsg = fmt.Sprintf("Fetching %d commits...", m.limit)
			return m, m.reloadCommitsCmd()

		case "-":
			if m.limit == 0 {
				m.limit = len(m.rawSnapshots) - 10
				if m.limit < 10 {
					m.limit = 10
				}
			} else if m.limit > 10 {
				m.limit -= 10
			}
			m.loading = true
			m.statusMsg = fmt.Sprintf("Fetching %d commits...", m.limit)
			return m, m.reloadCommitsCmd()

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
		m.scrollOffset = 0
		return
	}
	visibleRows := m.maxVisibleListRows()
	if visibleRows <= 0 {
		visibleRows = 8
	}

	if m.selectedIndex < m.scrollOffset {
		m.scrollOffset = m.selectedIndex
	}
	if m.selectedIndex >= m.scrollOffset+visibleRows {
		m.scrollOffset = m.selectedIndex - visibleRows + 1
	}

	maxScroll := len(m.snapshots) - visibleRows
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.scrollOffset > maxScroll {
		m.scrollOffset = maxScroll
	}
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
}

func (m Model) globalMaxValue() int {
	maxVal := 0
	for _, snap := range m.rawSnapshots {
		if v := snap.Total.Value(m.activeMetric); v > maxVal {
			maxVal = v
		}
	}
	for _, snap := range m.snapshots {
		if v := snap.Total.Value(m.activeMetric); v > maxVal {
			maxVal = v
		}
	}
	if maxVal <= 0 {
		maxVal = 1
	}
	return maxVal
}

// truncateStr truncates a string s to at most maxLen visual cells.
func truncateStr(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	w := lipgloss.Width(s)
	if w <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return "..."[:maxLen]
	}
	runes := []rune(s)
	for len(runes) > 0 && lipgloss.Width(string(runes)) > maxLen-3 {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + "..."
}

func (m Model) maxVisibleListRows() int {
	// Fixed line budget:
	// Header: 2 lines + 1 blank line = 3 lines
	// Card: 14 lines + 1 blank line = 15 lines
	// Legend: 1 line + 1 blank line = 2 lines
	// Footer: 1 line
	// Graph header row: 1 line
	// Graph blank line after: 1 line
	// Buffer: 1 line
	// Total non-bar overhead = 3 + 15 + 2 + 1 + 1 + 1 + 1 = 24 lines!
	avail := m.height - 24
	if avail < 5 {
		return 5
	}
	if avail > 18 {
		return 18
	}
	return avail
}

func (m Model) loadNextChunkCmd() tea.Cmd {
	if len(m.pendingInfos) == 0 {
		return nil
	}
	chunkSize := 25
	totalPending := len(m.pendingInfos)
	chunkStart := totalPending - chunkSize
	if chunkStart < 0 {
		chunkStart = 0
	}
	chunkInfos := make([]CommitInfo, len(m.pendingInfos[chunkStart:]))
	copy(chunkInfos, m.pendingInfos[chunkStart:])
	remainingPending := make([]CommitInfo, chunkStart)
	copy(remainingPending, m.pendingInfos[:chunkStart])

	repoRoot := m.repoRoot
	return func() tea.Msg {
		chunkSnaps, err := AnalyzeCommitBatch(repoRoot, chunkInfos, nil)
		return chunkLoadedMsg{
			snapshots:        chunkSnaps,
			remainingPending: remainingPending,
			err:              err,
		}
	}
}

func (m Model) reloadCommitsCmd() tea.Cmd {
	limit := m.limit
	repoRoot := m.repoRoot
	return func() tea.Msg {
		res, err := CollectInitialHistory(repoRoot, limit, 25, nil)
		if err != nil {
			return reloadMsg{err: err}
		}
		return reloadMsg{
			snapshots:    res.InitialSnapshots,
			pendingInfos: res.PendingInfos,
			totalCommits: res.TotalCommits,
		}
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

	// Master Failsafe: Ensure total rendered lines strictly <= m.height,
	// completely eliminating any terminal scrolling or header jitter!
	output := sb.String()
	lines := strings.Split(output, "\n")
	if m.height > 0 && len(lines) >= m.height {
		lines = lines[:m.height-1]
		output = strings.Join(lines, "\n")
	}
	return output
}

func (m Model) renderHeader() string {
	logo := titleStyle.Render("TOKEI-TIME")
	repoName := strings.TrimPrefix(m.repoRoot, "/Users/")
	if len(repoName) > 24 {
		repoName = "..." + repoName[len(repoName)-21:]
	}

	var countLabel string
	if m.isLazyLoading && m.totalCommits > 0 {
		pct := float64(m.loadedCommits) / float64(m.totalCommits) * 100
		countLabel = fmt.Sprintf("%d/%d commits (%.0f%%)", m.loadedCommits, m.totalCommits, pct)
	} else if m.activeGroup == GroupCommit {
		countLabel = fmt.Sprintf("%d commits", len(m.rawSnapshots))
	} else {
		countLabel = fmt.Sprintf("%d commits, %d %ss", len(m.rawSnapshots), len(m.snapshots), strings.ToLower(GroupModeNames[m.activeGroup]))
	}
	sub := subTitleStyle.Render(fmt.Sprintf("%s (%s)", repoName, countLabel))

	var tabs []string
	for _, metric := range MetricList {
		name := MetricNames[metric]
		if metric == m.activeMetric {
			tabs = append(tabs, activeTabStyle.Render(fmt.Sprintf(" %s ", name)))
		} else {
			tabs = append(tabs, inactiveTabStyle.Render(fmt.Sprintf(" %s ", name)))
		}
	}

	line1 := logo + "  " + sub + "  " + strings.Join(tabs, " ")

	var groupTabs []string
	for _, grp := range GroupModeList {
		name := GroupModeNames[grp]
		if grp == m.activeGroup {
			groupTabs = append(groupTabs, activeGroupTabStyle.Render(fmt.Sprintf(" %s ", name)))
		} else {
			groupTabs = append(groupTabs, inactiveGroupTabStyle.Render(fmt.Sprintf(" %s ", name)))
		}
	}
	line2 := dimStyle.Render("Summary: ") + strings.Join(groupTabs, " ") + dimStyle.Render("  (press 's' or 'g' to cycle)")

	if m.width > 0 {
		line1 = truncateStr(line1, m.width-1)
		line2 = truncateStr(line2, m.width-1)
	}
	return line1 + "\n" + line2
}

func (m Model) renderHorizontalView() string {
	var sb strings.Builder

	maxValue := m.globalMaxValue()

	visibleRows := m.maxVisibleListRows()
	start := m.scrollOffset
	end := start + visibleRows
	if end > len(m.snapshots) {
		end = len(m.snapshots)
	}

	col2Width := 10
	col3Width := 14

	// Dynamic column 4 width based on terminal width
	col4Width := 20
	if m.width > 0 && m.width < 80 {
		col4Width = 20 - (80 - m.width)
		if col4Width < 8 {
			col4Width = 8
		}
	}

	availBar := m.width - (51 + col4Width)
	if availBar < 6 {
		availBar = 6
	}
	if availBar > 55 {
		availBar = 55
	}
	barWidth := availBar

	diffLabel := "Δ Prev"
	if m.diffMode == DiffLatest {
		diffLabel = "Δ Latest"
	}

	var headerRow string
	if m.activeGroup == GroupCommit {
		headerRow = fmt.Sprintf("  %-10s %-14s %-*s %-*s %10s %8s",
			"COMMIT", "WHEN", col4Width, "SUBJECT", barWidth, "BREAKDOWN", strings.ToUpper(MetricNames[m.activeMetric]), diffLabel)
	} else {
		groupLabel := strings.ToUpper(GroupModeNames[m.activeGroup])
		headerRow = fmt.Sprintf("  %-10s %-14s %-*s %-*s %10s %8s",
			truncateStr(groupLabel, 10), "COMMITS", col4Width, "SUMMARY / SPAN", barWidth, "BREAKDOWN", "AVG "+strings.ToUpper(MetricNames[m.activeMetric]), diffLabel)
	}
	if m.width > 0 {
		headerRow = truncateStr(headerRow, m.width-1)
	}
	sb.WriteString(dimStyle.Render(headerRow))
	sb.WriteString("\n")

	renderedCount := 0
	for i := start; i < end; i++ {
		snap := m.snapshots[i]
		isSelected := i == m.selectedIndex

		cursor := "  "
		if isSelected {
			cursor = accentStyle.Render("▶ ")
		}
		var col2, col3, col4 string
		if snap.IsSummary {
			pLabel := snap.PeriodLabel
			if len(pLabel) > col2Width {
				pLabel = pLabel[:col2Width]
			}
			col2 = fmt.Sprintf("%-*s", col2Width, pLabel)
			if isSelected {
				col2 = selectedRowStyle.Render(col2)
			} else {
				col2 = dimStyle.Render(col2)
			}

			cmts := fmt.Sprintf("%d commits", snap.CommitCount)
			if len(cmts) > col3Width {
				cmts = fmt.Sprintf("%d cmts", snap.CommitCount)
			}
			col3 = dimStyle.Render(fmt.Sprintf("%-*s", col3Width, cmts))
			span := snap.RelativeDate
			if span == "" {
				span = snap.Subject
			}
			if len(span) > col4Width {
				span = span[:col4Width-1] + "…"
			}
			col4 = fmt.Sprintf("%-*s", col4Width, span)
			if isSelected {
				col4 = lipgloss.NewStyle().Bold(true).Render(col4)
			} else {
				col4 = dimStyle.Render(col4)
			}
		} else {
			hash := snap.ShortHash
			isHead := !snap.IsSummary && (i == len(m.snapshots)-1 || (i == len(m.snapshots)-2 && m.snapshots[len(m.snapshots)-1].IsWorkingTree))
			if snap.IsWorkingTree {
				col2 = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFAF00")).Bold(true).Render(fmt.Sprintf("%-*s", col2Width, "[WT]"))
			} else if isHead {
				if len(hash) > col2Width {
					hash = hash[:col2Width]
				}
				headStr := fmt.Sprintf("%-*s", col2Width, hash)
				col2 = lipgloss.NewStyle().Foreground(lipgloss.Color("#00D7D7")).Bold(true).Render(headStr)
			} else {
				if len(hash) > col2Width {
					hash = hash[:col2Width]
				}
				col2 = fmt.Sprintf("%-*s", col2Width, hash)
				if isSelected {
					col2 = selectedRowStyle.Render(col2)
				} else {
					col2 = dimStyle.Render(col2)
				}
			}

			timeAgo := snap.RelativeDate
			if snap.IsWorkingTree {
				timeAgo = "uncommitted"
			}
			if len(timeAgo) > col3Width {
				timeAgo = timeAgo[:col3Width]
			}
			col3 = dimStyle.Render(fmt.Sprintf("%-*s", col3Width, timeAgo))
			subj := snap.Subject
			if len(subj) > col4Width {
				subj = subj[:col4Width-1] + "…"
			}
			col4 = fmt.Sprintf("%-*s", col4Width, subj)
			if isSelected {
				col4 = lipgloss.NewStyle().Bold(true).Render(col4)
			} else {
				col4 = dimStyle.Render(col4)
			}
		}

		totalVal, bWidth, segments := CalculateHorizontalSegments(snap, m.activeMetric, barWidth, maxValue)

		var barBuf strings.Builder
		for _, seg := range segments {
			style := lipgloss.NewStyle().Foreground(lipgloss.Color(seg.Color))
			barBuf.WriteString(style.Render(strings.Repeat("█", seg.Chars)))
		}
		if bWidth < barWidth {
			barBuf.WriteString(dimStyle.Render(strings.Repeat("·", barWidth-bWidth)))
		}
		valStr := formatCompactNumber(totalVal)
		valStyled := fmt.Sprintf("%10s", valStr)
		if isSelected {
			valStyled = lipgloss.NewStyle().Bold(true).Render(valStyled)
		}

		delta := snap.TotalDiffPrev.Value(m.activeMetric)
		if m.diffMode == DiffLatest {
			delta = snap.TotalDiffLatest.Value(m.activeMetric)
		}

		var deltaStyled string
		if snap.IsWorkingTree && delta == 0 && snap.LinesAdded == 0 && snap.LinesDeleted == 0 {
			deltaStyled = dimStyle.Render("       -")
		} else if delta > 0 {
			deltaStyled = successStyle.Render(fmt.Sprintf("%+8d", delta))
		} else if delta < 0 {
			deltaStyled = dangerStyle.Render(fmt.Sprintf("%+8d", delta))
		} else {
			deltaStyled = dimStyle.Render("       0")
		}

		row := fmt.Sprintf("%s%s %s %s %s %s %s",
			cursor, col2, col3, col4, barBuf.String(), valStyled, deltaStyled)
		if m.width > 0 {
			row = truncateStr(row, m.width-1)
		}
		sb.WriteString(row + "\n")
		renderedCount++
	}

	// Pad remaining rows up to visibleRows so graph height NEVER changes!
	for range visibleRows - renderedCount {
		sb.WriteString("\n")
	}

	return sb.String()
}

func (m Model) renderVerticalView() string {
	var sb strings.Builder

	chartHeight := 10
	maxValue := m.globalMaxValue()

	// Fixed number of columns in the vertical chart view
	availCols := (m.width - 12) / 4
	if availCols < 8 {
		availCols = 8
	}
	if availCols > 24 {
		availCols = 24
	}

	startCol := 0
	if len(m.snapshots) > availCols {
		startCol = m.selectedIndex - availCols/2
		if startCol < 0 {
			startCol = 0
		}
		if startCol > len(m.snapshots)-availCols {
			startCol = len(m.snapshots) - availCols
		}
	}
	endCol := startCol + availCols
	if endCol > len(m.snapshots) {
		endCol = len(m.snapshots)
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

		renderedCols := 0
		for c := startCol; c < endCol; c++ {
			col := cols[c]
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
			renderedCols++
		}
		for renderedCols < availCols {
			sb.WriteString("    ")
			renderedCols++
		}
		sb.WriteString("\n")
	}

	// X-axis border
	sb.WriteString(dimStyle.Render("        ┴"))
	for range availCols {
		sb.WriteString(dimStyle.Render("────"))
	}
	sb.WriteString("\n")

	// Selector marker row
	sb.WriteString("         ")
	renderedSel := 0
	for c := startCol; c < endCol; c++ {
		if c == m.selectedIndex {
			sb.WriteString(accentStyle.Render(" ▲  "))
		} else {
			sb.WriteString("    ")
		}
		renderedSel++
	}
	for renderedSel < availCols {
		sb.WriteString("    ")
		renderedSel++
	}
	sb.WriteString("\n")

	// Commit labels row
	sb.WriteString("         ")
	renderedLabels := 0
	for c := startCol; c < endCol; c++ {
		snap := m.snapshots[c]
		var label string
		if snap.IsSummary {
			switch m.activeGroup {
			case GroupDay:
				parts := strings.Split(snap.PeriodLabel, "-")
				if len(parts) >= 3 {
					label = parts[1] + "/" + parts[2]
				} else {
					label = snap.PeriodLabel
				}
			case GroupWeek:
				if idx := strings.Index(snap.PeriodLabel, "W"); idx >= 0 {
					label = snap.PeriodLabel[idx:]
				} else {
					label = snap.PeriodLabel
				}
			case GroupMonth:
				fields := strings.Fields(snap.PeriodLabel)
				if len(fields) > 0 {
					label = fields[0]
				} else {
					label = snap.PeriodLabel
				}
			default:
				label = snap.PeriodLabel
			}
			if len(label) > 4 {
				label = label[:4]
			}
			label = fmt.Sprintf("%-4s", label)
		} else {
			raw := snap.ShortHash
			if snap.IsWorkingTree {
				raw = " WT"
			} else if len(raw) > 3 {
				raw = raw[:3]
			}
			label = fmt.Sprintf("%-3s ", raw)
		}
		if c == m.selectedIndex {
			sb.WriteString(selectedRowStyle.Render(label))
		} else {
			sb.WriteString(dimStyle.Render(label))
		}
		renderedLabels++
	}
	for renderedLabels < availCols {
		sb.WriteString("    ")
		renderedLabels++
	}
	sb.WriteString("\n")

	return sb.String()
}

func (m Model) renderDetailCard() string {
	snap := m.snapshots[m.selectedIndex]

	cardWidth := m.width - 2
	if cardWidth < 50 {
		cardWidth = 50
	}
	contentWidth := cardWidth - 4

	var lines []string

	// Header line
	var title string
	if snap.IsSummary {
		title = fmt.Sprintf("Summary: %s (%d commits)", snap.PeriodLabel, snap.CommitCount)
		if snap.IsWorkingTree {
			title += " [includes uncommitted working tree]"
		}
	} else if snap.IsWorkingTree {
		title = "Working Tree (Uncommitted Changes)"
	} else {
		title = fmt.Sprintf("Commit: %s", snap.Hash)
	}
	lines = append(lines, accentStyle.Render(truncateStr(title, contentWidth)))

	var meta string
	if snap.IsSummary {
		meta = fmt.Sprintf("Span: %s • %s", snap.Date, snap.Subject)
	} else {
		meta = fmt.Sprintf("Author: %s • %s (%s)", snap.Author, snap.RelativeDate, snap.Date)
		if snap.Subject != "" {
			meta += fmt.Sprintf(" • Subject: %s", snap.Subject)
		}
	}
	lines = append(lines, dimStyle.Render(truncateStr(meta, contentWidth)))

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

	labelPrefix := "Total"
	if snap.IsSummary {
		labelPrefix = "Avg"
	}
	statLine := fmt.Sprintf("%s %s: %s   Δ vs Previous: %s   Δ vs Latest: %s",
		labelPrefix, metricName, accentStyle.Render(formatNumber(totalVal)), diffPrevStr, diffLatestStr)

	if snap.LinesAdded > 0 || snap.LinesDeleted > 0 {
		statLine += fmt.Sprintf("   git: %s / %s lines",
			successStyle.Render(fmt.Sprintf("+%d", snap.LinesAdded)),
			dangerStyle.Render(fmt.Sprintf("-%d", snap.LinesDeleted)))
	}
	lines = append(lines, truncateStr(statLine, contentWidth))

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
	colMetricHeader := metricName
	filesHeader := "FILES"
	linesHeader := "LINES"
	if snap.IsSummary {
		colMetricHeader = "AVG " + strings.ToUpper(metricName)
		if len(colMetricHeader) > 9 {
			colMetricHeader = colMetricHeader[:9]
		}
		filesHeader = "AVG FILES"
		linesHeader = "AVG LINES"
	}
	tableHeader := fmt.Sprintf("  %-14s %9s %7s %9s %9s %9s %9s",
		"LANGUAGE", colMetricHeader, "SHARE", "Δ PREV", "Δ LATEST", filesHeader, linesHeader)
	tableLines = append(tableLines, dimStyle.Render(truncateStr(tableHeader, contentWidth)))

	const fixedTableRows = 6
	limitRows := len(rows)
	if limitRows > fixedTableRows {
		limitRows = fixedTableRows
	}

	for i := range limitRows {
		r := rows[i]
		color := GetLanguageColor(r.name)
		icon := lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render("■")

		rawDp := FormatDelta(r.deltaPrev)
		var dpStr string
		if r.deltaPrev > 0 {
			dpStr = successStyle.Render(fmt.Sprintf("%9s", rawDp))
		} else if r.deltaPrev < 0 {
			dpStr = dangerStyle.Render(fmt.Sprintf("%9s", rawDp))
		} else {
			dpStr = dimStyle.Render(fmt.Sprintf("%9s", rawDp))
		}

		rawDl := FormatDelta(r.deltaLatest)
		var dlStr string
		if r.deltaLatest > 0 {
			dlStr = successStyle.Render(fmt.Sprintf("%9s", rawDl))
		} else if r.deltaLatest < 0 {
			dlStr = dangerStyle.Render(fmt.Sprintf("%9s", rawDl))
		} else {
			dlStr = dimStyle.Render(fmt.Sprintf("%9s", rawDl))
		}

		line := fmt.Sprintf("  %s %-12s %9s %6.1f%% %s %s %9d %9s",
			icon, truncateStr(r.name, 12), formatCompactNumber(r.val), r.pct, dpStr, dlStr, r.files, formatCompactNumber(r.lines))
		tableLines = append(tableLines, truncateStr(line, contentWidth))
	}

	// Pad remaining rows up to fixedTableRows so card height NEVER changes when scrolling!
	for len(tableLines)-1 < fixedTableRows {
		tableLines = append(tableLines, dimStyle.Render("    -"))
	}

	if len(rows) > fixedTableRows {
		tableLines = append(tableLines, dimStyle.Render(truncateStr(fmt.Sprintf("  ... and %d more languages", len(rows)-fixedTableRows), contentWidth)))
	} else {
		tableLines = append(tableLines, "")
	}

	if snap.IsSummary && len(snap.SubCommits) > 0 {
		var commitListStr string
		if len(snap.SubCommits) <= 4 {
			commitListStr = strings.Join(snap.SubCommits, ", ")
		} else {
			commitListStr = fmt.Sprintf("%s, ... (+%d more)", strings.Join(snap.SubCommits[:3], ", "), len(snap.SubCommits)-3)
		}
		tableLines = append(tableLines, dimStyle.Render(truncateStr("  Commits in period: "+commitListStr, contentWidth)))
	} else if !snap.IsSummary && !snap.IsWorkingTree {
		tableLines = append(tableLines, dimStyle.Render(truncateStr("  Commit Hash: "+snap.Hash, contentWidth)))
	} else {
		tableLines = append(tableLines, dimStyle.Render(truncateStr("  Working tree changes on disk", contentWidth)))
	}
	lines = append(lines, strings.Join(tableLines, "\n"))

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
	res := dimStyle.Render("Languages: ") + legend
	if m.width > 0 {
		res = truncateStr(res, m.width-2)
	}
	return res
}

func (m Model) renderFooter() string {
	keys := []string{
		accentStyle.Render("Tab") + ": Metric",
		accentStyle.Render("s") + ": Summary (Day/Wk/Mo/Yr)",
		accentStyle.Render("↑/↓") + ": Nav",
		accentStyle.Render("v") + ": View",
		accentStyle.Render("c") + ": Diff",
		accentStyle.Render("+/-") + ": Commits",
		accentStyle.Render("a") + ": All",
		accentStyle.Render("q") + ": Quit",
	}

	bar := strings.Join(keys, " • ")
	if m.statusMsg != "" {
		bar = bar + "  " + dimStyle.Render(m.statusMsg)
	}
	if m.width > 0 {
		bar = truncateStr(bar, m.width-2)
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
