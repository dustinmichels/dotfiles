package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dustinmichels/agent-ls/models"
	"github.com/dustinmichels/agent-ls/providers"
	"github.com/dustinmichels/agent-ls/storage"
)

var (
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4")).
			Padding(0, 1)

	cardStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#555555")).
			Padding(0, 1).
			MarginBottom(1)

	accentStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#00D7D7"))

	warningStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFAF00"))

	successStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#00AF5F"))

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#777777"))

	searchPromptStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FFAF00"))

	selectedRowStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#00D7D7"))

	modalStyle = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(lipgloss.Color("#FF5F87")).
			Padding(1, 2).
			Bold(true)
)

type Model struct {
	manager           *providers.Manager
	allSessions       []models.Session
	filtered          []models.Session
	selected          map[string]bool
	table             table.Model
	searchInput       textinput.Model
	searching         bool
	toolFilter        string
	olderThan14dOnly  bool
	cutoff            time.Time
	width             int
	height            int
	showConfirmDelete bool
	statusMsg         string
	statusIsError     bool
	showHelp          bool
}

func NewModel(mgr *providers.Manager, sessions []models.Session, initialTool string) Model {
	ti := textinput.New()
	ti.Placeholder = "Filter by project, title, or id..."
	ti.Prompt = " / "
	ti.PromptStyle = searchPromptStyle
	ti.CharLimit = 64
	ti.Width = 32

	now := time.Now()
	cutoff := now.Add(-14 * 24 * time.Hour)

	m := Model{
		manager:          mgr,
		allSessions:      sessions,
		selected:         make(map[string]bool),
		searchInput:      ti,
		toolFilter:       initialTool,
		olderThan14dOnly: false,
		cutoff:           cutoff,
		width:            100,
		height:           28,
	}

	m.applyFilters()
	m.initTable()
	return m
}

func (m *Model) initTable() {
	columns := []table.Column{
		{Title: "SEL", Width: 4},
		{Title: "TOOL", Width: 12},
		{Title: "PROJECT", Width: 16},
		{Title: "TITLE / SUMMARY", Width: 42},
		{Title: "SIZE", Width: 10},
		{Title: "AGE", Width: 10},
		{Title: "STATUS", Width: 12},
	}

	now := time.Now()
	rows := make([]table.Row, len(m.filtered))
	for i, s := range m.filtered {
		sel := "[ ]"
		if m.selected[s.ID] {
			sel = "[x]"
		}

		toolName := s.Tool
		if s.IsArchived {
			toolName = s.Tool + " (arch)"
		}

		status := "active"
		if s.IsLive {
			status = "LIVE"
		} else if s.IsOlderThan(m.cutoff) {
			status = ">14d"
		}

		rows[i] = table.Row{
			sel,
			toolName,
			s.Project,
			s.Title,
			storage.FormatBytes(s.AllocatedBytes),
			storage.FormatDuration(s.Age(now)),
			status,
		}
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(m.tableHeight()),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(true)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(s)

	m.table = t
}

func (m *Model) tableHeight() int {
	h := m.height - 13
	if h < 6 {
		return 6
	}
	return h
}

func (m *Model) applyFilters() {
	query := strings.ToLower(strings.TrimSpace(m.searchInput.Value()))
	var filtered []models.Session

	for _, s := range m.allSessions {
		if m.toolFilter != "" && m.toolFilter != "all" && s.Tool != m.toolFilter {
			continue
		}
		if m.olderThan14dOnly && !s.IsOlderThan(m.cutoff) {
			continue
		}
		if query != "" {
			projMatch := strings.Contains(strings.ToLower(s.Project), query)
			titleMatch := strings.Contains(strings.ToLower(s.Title), query)
			idMatch := strings.Contains(strings.ToLower(s.ID), query)
			if !projMatch && !titleMatch && !idMatch {
				continue
			}
		}
		filtered = append(filtered, s)
	}

	m.filtered = filtered
}

func (m *Model) selectedStats() (int, int64) {
	count := 0
	var bytes int64
	for _, s := range m.allSessions {
		if m.selected[s.ID] {
			count++
			bytes += s.AllocatedBytes
		}
	}
	return count, bytes
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.initTable()

	case tea.KeyMsg:
		if m.showConfirmDelete {
			switch msg.String() {
			case "y", "Y", "enter":
				m.executeDeletion()
				m.showConfirmDelete = false
			case "n", "N", "esc", "q":
				m.showConfirmDelete = false
			}
			return m, nil
		}

		if m.searching {
			switch msg.String() {
			case "enter", "esc":
				m.searching = false
				m.searchInput.Blur()
			default:
				m.searchInput, cmd = m.searchInput.Update(msg)
				m.applyFilters()
				m.initTable()
				return m, cmd
			}
			return m, nil
		}

		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "/":
			m.searching = true
			m.searchInput.Focus()
			return m, textinput.Blink

		case "esc":
			if m.searchInput.Value() != "" {
				m.searchInput.SetValue("")
				m.applyFilters()
				m.initTable()
			}

		case "t":
			// Cycle tool
			tools := []string{"all", "omp", "antigravity", "claude", "codex"}
			currIdx := 0
			for i, t := range tools {
				if t == m.toolFilter {
					currIdx = i
					break
				}
			}
			m.toolFilter = tools[(currIdx+1)%len(tools)]
			m.applyFilters()
			m.initTable()

		case "1":
			m.toolFilter = "omp"
			m.applyFilters()
			m.initTable()
		case "2":
			m.toolFilter = "antigravity"
			m.applyFilters()
			m.initTable()
		case "3":
			m.toolFilter = "claude"
			m.applyFilters()
			m.initTable()
		case "4":
			m.toolFilter = "codex"
			m.applyFilters()
			m.initTable()
		case "0":
			m.toolFilter = "all"
			m.applyFilters()
			m.initTable()

		case "o":
			m.olderThan14dOnly = !m.olderThan14dOnly
			m.applyFilters()
			m.initTable()

		case "O":
			// Shift+O: Select all >14d in current filtered view (omp and antigravity only)
			allAlreadySelected := true
			for _, s := range m.filtered {
				if (s.Tool == "omp" || s.Tool == "antigravity") && s.IsOlderThan(m.cutoff) && !m.selected[s.ID] {
					allAlreadySelected = false
					break
				}
			}
			for _, s := range m.filtered {
				if (s.Tool == "omp" || s.Tool == "antigravity") && s.IsOlderThan(m.cutoff) {
					m.selected[s.ID] = !allAlreadySelected
				}
			}
			m.initTable()

		case " ":
			// Toggle select on cursor row
			idx := m.table.Cursor()
			if idx >= 0 && idx < len(m.filtered) {
				s := m.filtered[idx]
				if s.Tool != "omp" && s.Tool != "antigravity" {
					m.statusMsg = fmt.Sprintf("%s sessions are list-only (Claude self-prunes, Codex manages lifecycle)", s.Tool)
					m.statusIsError = true
					return m, nil
				}
				m.selected[s.ID] = !m.selected[s.ID]
				m.initTable()
			}

		case "a":
			// Toggle select all visible (omp and antigravity only)
			allSel := true
			for _, s := range m.filtered {
				if (s.Tool == "omp" || s.Tool == "antigravity") && !m.selected[s.ID] {
					allSel = false
					break
				}
			}
			for _, s := range m.filtered {
				if s.Tool == "omp" || s.Tool == "antigravity" {
					m.selected[s.ID] = !allSel
				}
			}
			m.initTable()

		case "d", "x":
			count, _ := m.selectedStats()
			if count > 0 {
				m.showConfirmDelete = true
			} else {
				m.statusMsg = "No sessions selected. Press Space or 'O' to select sessions."
				m.statusIsError = true
			}

		case "r":
			// Rescan
			if sessions, err := m.manager.ScanAll(""); err == nil {
				m.allSessions = sessions
				m.applyFilters()
				m.initTable()
				m.statusMsg = "Rescanned all sessions."
				m.statusIsError = false
			}

		case "?":
			m.showHelp = !m.showHelp
		}
	}

	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m *Model) executeDeletion() {
	var toDelete []models.Session
	hasOmp := false

	for _, s := range m.allSessions {
		if m.selected[s.ID] {
			toDelete = append(toDelete, s)
			if s.Tool == "omp" {
				hasOmp = true
			}
		}
	}

	deletedCount := 0
	var freedBytes int64
	var errs []string

	for _, s := range toDelete {
		if err := m.manager.DeleteSession(s); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", s.ID, err))
		} else {
			deletedCount++
			freedBytes += s.AllocatedBytes
			delete(m.selected, s.ID)
		}
	}

	if hasOmp {
		_ = providers.RunOmpGC()
	}

	// Rescan
	if sessions, err := m.manager.ScanAll(""); err == nil {
		m.allSessions = sessions
	}
	m.applyFilters()
	m.initTable()

	if len(errs) > 0 {
		m.statusMsg = fmt.Sprintf("Cleaned %d sessions (%s freed), %d errors: %s",
			deletedCount, storage.FormatBytes(freedBytes), len(errs), strings.Join(errs, "; "))
		m.statusIsError = true
	} else {
		m.statusMsg = fmt.Sprintf("✨ Successfully moved %d sessions to Trash (%s freed)!",
			deletedCount, storage.FormatBytes(freedBytes))
		m.statusIsError = false
	}
}

func (m Model) View() string {
	var b strings.Builder

	// 1. Agent Overview Card
	summaries := m.manager.Summaries(m.allSessions, m.cutoff)
	var totalCount, totalOldCount int
	var totalBytes, totalOldBytes int64

	agentLines := []string{headerStyle.Render("AGENT OVERVIEW:")}
	for i, s := range summaries {
		totalCount += s.TotalCount
		totalBytes += s.TotalBytes
		totalOldCount += s.OldCount
		totalOldBytes += s.OldBytes

		numKey := fmt.Sprintf("[%d]", i+1)
		line := fmt.Sprintf(" %s %-12s: %5d sessions · %8s  (>14d: %5d sessions · %8s)",
			warningStyle.Render(numKey),
			s.Tool,
			s.TotalCount,
			storage.FormatBytes(s.TotalBytes),
			s.OldCount,
			storage.FormatBytes(s.OldBytes),
		)
		agentLines = append(agentLines, line)
	}

	totalLine := fmt.Sprintf(" %-16s: %5d sessions · %8s  (>14d: %5d sessions · %8s)",
		accentStyle.Render("TOTAL"),
		totalCount,
		storage.FormatBytes(totalBytes),
		totalOldCount,
		storage.FormatBytes(totalOldBytes),
	)
	agentLines = append(agentLines, totalLine)
	b.WriteString(cardStyle.Render(strings.Join(agentLines, "\n")))
	b.WriteString("\n")

	// 2. Filter & Selection status bar
	selCount, selBytes := m.selectedStats()
	toolLabel := m.toolFilter
	if toolLabel == "" || toolLabel == "all" {
		toolLabel = "All Agents"
	}

	viewLabel := "All Sessions"
	if m.olderThan14dOnly {
		viewLabel = warningStyle.Render(">14 Days Only")
	}

	selStatus := dimStyle.Render("0 selected")
	if selCount > 0 {
		selStatus = successStyle.Render(fmt.Sprintf("%d selected · %s space savings", selCount, storage.FormatBytes(selBytes)))
	}

	controls := fmt.Sprintf("Filter: %s (t)  View: %s (o)  Select >14d: [O]  Selected: %s",
		accentStyle.Render(toolLabel),
		viewLabel,
		selStatus,
	)
	b.WriteString(controls)
	b.WriteString("\n")

	// Search bar if active or populated
	if m.searching || m.searchInput.Value() != "" {
		b.WriteString(m.searchInput.View())
		b.WriteString("\n")
	}

	// 3. Table
	b.WriteString(m.table.View())
	b.WriteString("\n")

	// 4. Modal or Status line
	if m.showConfirmDelete {
		cnt, sz := m.selectedStats()
		modal := modalStyle.Render(fmt.Sprintf(
			"🗑️  TRASH SESSIONS CONFIRMATION\n\n"+
				"Are you sure you want to move %d session(s) to Trash?\n"+
				"Reclaimable disk space: %s\n\n"+
				"Sessions will be moved reversibly to ~/.Trash/agent-ls/\n\n"+
				"[y] Confirm Trash    [n] Cancel",
			cnt, storage.FormatBytes(sz),
		))
		b.WriteString("\n" + modal + "\n")
	} else if m.statusMsg != "" {
		if m.statusIsError {
			b.WriteString(warningStyle.Render("⚠ " + m.statusMsg))
		} else {
			b.WriteString(successStyle.Render(m.statusMsg))
		}
		b.WriteString("\n")
	} else {
		helpText := dimStyle.Render("[↑/↓] Navigate  [Space] Select  [O] Select All >14d  [a] Toggle Visible  [d] Trash  [/] Search  [q] Quit")
		b.WriteString(helpText)
		b.WriteString("\n")
	}

	return b.String()
}
