package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

// PackageItem represents a single formula or cask.
type PackageItem struct {
	Name        string    `json:"name"`
	FullName    string    `json:"full_name,omitempty"`
	Type        string    `json:"type"` // "formula" or "cask"
	Version     string    `json:"version"`
	Desc        string    `json:"desc"`
	Homepage    string    `json:"homepage"`
	InstalledAt time.Time `json:"installed_at"`
	Epoch       int64     `json:"epoch"`
	RequiredBy  []string  `json:"required_by,omitempty"`
}

type BrewJSON struct {
	Formulae []Formula `json:"formulae"`
	Casks    []Cask    `json:"casks"`
}

type RuntimeDep struct {
	FullName string `json:"full_name"`
	Version  string `json:"version"`
}

type Formula struct {
	Name         string      `json:"name"`
	FullName     string      `json:"full_name"`
	Desc         string      `json:"desc"`
	Homepage     string      `json:"homepage"`
	Dependencies []string    `json:"dependencies"`
	Installed    []Installed `json:"installed"`
}

type Installed struct {
	Version             string       `json:"version"`
	Time                int64        `json:"time"`
	InstalledOnRequest  bool         `json:"installed_on_request"`
	RuntimeDependencies []RuntimeDep `json:"runtime_dependencies"`
}

type Cask struct {
	Token         string   `json:"token"`
	Name          []string `json:"name"`
	Desc          string   `json:"desc"`
	Homepage      string   `json:"homepage"`
	Version       string   `json:"version"`
	InstalledTime int64    `json:"installed_time"`
}

// Get Homebrew prefix.
func getBrewPrefix() string {
	out, err := exec.Command("brew", "--prefix").Output()
	if err == nil {
		p := strings.TrimSpace(string(out))
		if p != "" {
			return p
		}
	}
	if p := os.Getenv("HOMEBREW_PREFIX"); p != "" {
		return p
	}
	if _, err := os.Stat("/opt/homebrew"); err == nil {
		return "/opt/homebrew"
	}
	return "/usr/local"
}

// Read directory birthtimes on macOS APFS via syscall.Stat_t.Birthtimespec.
func readBirthTimes(dir string) map[string]int64 {
	results := make(map[string]int64)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return results
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		var stat syscall.Stat_t
		if err := syscall.Stat(path, &stat); err == nil {
			if stat.Birthtimespec.Sec > 0 {
				results[entry.Name()] = stat.Birthtimespec.Sec
			}
		}
	}
	return results
}

// Load all packages from Homebrew.
func loadPackages() ([]PackageItem, error) {
	prefix := getBrewPrefix()
	formulaBirths := readBirthTimes(filepath.Join(prefix, "Cellar"))
	caskBirths := readBirthTimes(filepath.Join(prefix, "Caskroom"))

	cmd := exec.Command("brew", "info", "--json=v2", "--installed")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run 'brew info': %w", err)
	}

	var data BrewJSON
	if err := json.Unmarshal(out, &data); err != nil {
		return nil, fmt.Errorf("failed to parse brew json: %w", err)
	}

	// Build reverse dependency map for all installed formulae using installed runtime dependencies.
	dependentsMap := make(map[string]map[string]struct{})
	recordDep := func(dep, dependent string) {
		dep = strings.TrimSpace(dep)
		if dep == "" || dep == dependent {
			return
		}
		if dependentsMap[dep] == nil {
			dependentsMap[dep] = make(map[string]struct{})
		}
		dependentsMap[dep][dependent] = struct{}{}
		if strings.Contains(dep, "/") {
			base := filepath.Base(dep)
			if dependentsMap[base] == nil {
				dependentsMap[base] = make(map[string]struct{})
			}
			dependentsMap[base][dependent] = struct{}{}
		}
	}

	for _, f := range data.Formulae {
		if len(f.Installed) == 0 {
			continue
		}
		lastInst := f.Installed[len(f.Installed)-1]
		for _, rDep := range lastInst.RuntimeDependencies {
			recordDep(rDep.FullName, f.Name)
		}
	}

	var items []PackageItem

	// Process Formulae
	for _, f := range data.Formulae {
		if len(f.Installed) == 0 {
			continue
		}
		// Check if installed on request
		isManual := false
		for _, inst := range f.Installed {
			if inst.InstalledOnRequest {
				isManual = true
				break
			}
		}
		if !isManual {
			continue
		}

		lastInst := f.Installed[len(f.Installed)-1]
		// Get birthtime, falling back to receipt time
		var epoch int64
		if b, ok := formulaBirths[f.Name]; ok && b > 0 {
			epoch = b
		} else {
			epoch = lastInst.Time
		}

		var requiredBy []string
		depSet := make(map[string]struct{})
		for dep := range dependentsMap[f.Name] {
			depSet[dep] = struct{}{}
		}
		if f.FullName != "" && f.FullName != f.Name {
			for dep := range dependentsMap[f.FullName] {
				depSet[dep] = struct{}{}
			}
		}
		for dep := range depSet {
			requiredBy = append(requiredBy, dep)
		}
		sort.Strings(requiredBy)

		items = append(items, PackageItem{
			Name:        f.Name,
			FullName:    f.FullName,
			Type:        "formula",
			Version:     lastInst.Version,
			Desc:        f.Desc,
			Homepage:    f.Homepage,
			InstalledAt: time.Unix(epoch, 0),
			Epoch:       epoch,
			RequiredBy:  requiredBy,
		})
	}

	// Process Casks (all casks are manual installations)
	for _, c := range data.Casks {
		var epoch int64
		if b, ok := caskBirths[c.Token]; ok && b > 0 {
			epoch = b
		} else {
			epoch = c.InstalledTime
		}

		items = append(items, PackageItem{
			Name:        c.Token,
			Type:        "cask",
			Version:     c.Version,
			Desc:        c.Desc,
			Homepage:    c.Homepage,
			InstalledAt: time.Unix(epoch, 0),
			Epoch:       epoch,
		})
	}

	return items, nil
}

// Filter and Sort Modes
type FilterType string

const (
	FilterAll     FilterType = "all"
	FilterFormula FilterType = "formula"
	FilterCask    FilterType = "cask"
)

type SortMode int

const (
	SortDateDesc SortMode = iota // Newest first
	SortDateAsc                  // Oldest first
	SortNameAsc                  // A-Z
)

// Modal Action States
type modalAction int

const (
	actionNone modalAction = iota
	actionConfirmDelete
	actionBlockedDelete
	actionConfirmUpgrade
)

type brewFinishedMsg struct {
	action   string // "upgrade" or "uninstall"
	itemName string
	err      error
}

func runBrewCommand(action string, item *PackageItem) tea.Cmd {
	var brewArgs []string
	if action == "upgrade" {
		brewArgs = []string{"upgrade"}
		if item.Type == "cask" {
			brewArgs = append(brewArgs, "--cask")
		}
		brewArgs = append(brewArgs, item.Name)
	} else {
		brewArgs = []string{"uninstall"}
		if item.Type == "cask" {
			brewArgs = append(brewArgs, "--cask")
		}
		brewArgs = append(brewArgs, item.Name)
	}

	var shScript string
	if action == "uninstall" {
		shScript = `brew "$@"; rc=$?; if [ $rc -ne 0 ]; then echo; read -r -p "Press Enter to return to brew-ls..." _; fi; exit $rc`
	} else {
		shScript = `brew "$@"; rc=$?; echo; read -r -p "Press Enter to return to brew-ls..." _; exit $rc`
	}
	args := append([]string{"-c", shScript, "_"}, brewArgs...)
	c := exec.Command("sh", args...)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr

	return tea.ExecProcess(c, func(err error) tea.Msg {
		return brewFinishedMsg{
			action:   action,
			itemName: item.Name,
			err:      err,
		}
	})
}

// UI Styles
var (
	subtleColor   = lipgloss.AdaptiveColor{Light: "#888888", Dark: "#777777"}
	accentColor   = lipgloss.AdaptiveColor{Light: "#5A56E0", Dark: "#7D56F4"}
	formulaColor  = lipgloss.Color("#22C55E")
	caskColor     = lipgloss.Color("#EC4899")
	selectedColor = lipgloss.Color("#38BDF8")

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(accentColor).
			Padding(0, 1)

	badgeFormula = lipgloss.NewStyle().
			Foreground(formulaColor).
			Bold(true)

	badgeCask = lipgloss.NewStyle().
			Foreground(caskColor).
			Bold(true)

	statusStyle = lipgloss.NewStyle().
			Foreground(subtleColor).
			Padding(0, 1)

	searchPromptStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#F59E0B")).
				Bold(true)

	modalBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(accentColor).
			Padding(1, 2).
			Width(68)

	tableSelectedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(selectedColor).
				Bold(true)

	caskRowStyle = lipgloss.NewStyle().
			Foreground(caskColor)
)

// Bubble Tea Model
type model struct {
	allItems      []PackageItem
	filtered      []PackageItem
	table         table.Model
	searchInput   textinput.Model
	searching     bool
	filterType    FilterType
	sortMode      SortMode
	width         int
	height        int
	showDetails   bool
	selectedItem  *PackageItem
	modalAction   modalAction
	statusMessage string
	statusIsError bool
	showHelp      bool
}

func initialModel(items []PackageItem, initialFilter FilterType) model {
	ti := textinput.New()
	ti.Placeholder = "Search packages..."
	ti.Prompt = " / "
	ti.PromptStyle = searchPromptStyle
	ti.CharLimit = 64
	ti.Width = 32

	m := model{
		allItems:    items,
		filterType:  initialFilter,
		sortMode:    SortDateDesc,
		searchInput: ti,
		width:       100,
		height:      28,
		modalAction: actionNone,
	}

	m.applyFilters()
	m.initTable()
	return m
}

func (m *model) applyFilters() {
	query := strings.ToLower(strings.TrimSpace(m.searchInput.Value()))

	var filtered []PackageItem
	for _, item := range m.allItems {
		if m.filterType == FilterFormula && item.Type != "formula" {
			continue
		}
		if m.filterType == FilterCask && item.Type != "cask" {
			continue
		}

		if query != "" {
			nameMatch := strings.Contains(strings.ToLower(item.Name), query)
			descMatch := strings.Contains(strings.ToLower(item.Desc), query)
			if !nameMatch && !descMatch {
				continue
			}
		}
		filtered = append(filtered, item)
	}

	// Sort
	switch m.sortMode {
	case SortDateDesc:
		sort.SliceStable(filtered, func(i, j int) bool {
			if filtered[i].Epoch != filtered[j].Epoch {
				return filtered[i].Epoch > filtered[j].Epoch
			}
			return strings.ToLower(filtered[i].Name) < strings.ToLower(filtered[j].Name)
		})
	case SortDateAsc:
		sort.SliceStable(filtered, func(i, j int) bool {
			if filtered[i].Epoch != filtered[j].Epoch {
				return filtered[i].Epoch < filtered[j].Epoch
			}
			return strings.ToLower(filtered[i].Name) < strings.ToLower(filtered[j].Name)
		})
	case SortNameAsc:
		sort.SliceStable(filtered, func(i, j int) bool {
			return strings.ToLower(filtered[i].Name) < strings.ToLower(filtered[j].Name)
		})
	}

	m.filtered = filtered
}

func (m *model) initTable() {
	// Calculate dynamic column widths
	maxName := 14
	maxVer := 10
	for _, it := range m.filtered {
		if len(it.Name) > maxName {
			maxName = len(it.Name)
		}
		if len(it.Version) > maxVer {
			maxVer = len(it.Version)
		}
	}
	if maxName > 28 {
		maxName = 28
	}
	if maxVer > 18 {
		maxVer = 18
	}

	availWidth := m.width
	if availWidth <= 0 {
		availWidth = 100
	}
	// Fixed widths: Date(17), Type(9), Name(maxName+2), Version(maxVer+2)
	fixedWidths := 17 + 9 + (maxName + 2) + (maxVer + 2) + 6
	descWidth := availWidth - fixedWidths
	if descWidth < 20 {
		descWidth = 20
	}

	columns := []table.Column{
		{Title: "FIRST INSTALLED", Width: 17},
		{Title: "TYPE", Width: 9},
		{Title: "NAME", Width: maxName},
		{Title: "VERSION", Width: maxVer},
		{Title: "DESCRIPTION", Width: descWidth},
	}

	var rows []table.Row
	for _, it := range m.filtered {
		dateStr := it.InstalledAt.Format("2006-01-02 15:04")
		if it.Epoch <= 0 {
			dateStr = "unknown"
		}
		rows = append(rows, table.Row{
			dateStr,
			it.Type,
			it.Name,
			it.Version,
			it.Desc,
		})
	}

	prevCursor := m.table.Cursor()

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
	s.Selected = tableSelectedStyle
	t.SetStyles(s)

	if len(rows) > 0 {
		if prevCursor >= len(rows) {
			prevCursor = len(rows) - 1
		}
		if prevCursor < 0 {
			prevCursor = 0
		}
		t.SetCursor(prevCursor)
	}

	m.table = t
}

func (m model) renderHeader() string {
	title := titleStyle.Render("brew-ls")

	var allBadge, formulaBadge, caskBadge string
	if m.filterType == FilterAll {
		allBadge = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).Background(lipgloss.Color("#2563EB")).Padding(0, 1).Render("All")
	} else {
		allBadge = lipgloss.NewStyle().Foreground(subtleColor).Padding(0, 1).Render("All")
	}

	if m.filterType == FilterFormula {
		formulaBadge = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).Background(formulaColor).Padding(0, 1).Render("Formulae")
	} else {
		formulaBadge = lipgloss.NewStyle().Foreground(subtleColor).Padding(0, 1).Render("Formulae")
	}

	if m.filterType == FilterCask {
		caskBadge = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).Background(caskColor).Padding(0, 1).Render("Casks")
	} else {
		caskBadge = lipgloss.NewStyle().Foreground(subtleColor).Padding(0, 1).Render("Casks")
	}

	filterTabs := lipgloss.JoinHorizontal(lipgloss.Center, allBadge, formulaBadge, caskBadge)

	var sortLabel string
	switch m.sortMode {
	case SortDateDesc:
		sortLabel = "📅 Newest first"
	case SortDateAsc:
		sortLabel = "📅 Oldest first"
	case SortNameAsc:
		sortLabel = "🔤 Name (A-Z)"
	}
	sortBadge := lipgloss.NewStyle().Foreground(subtleColor).Render("[" + sortLabel + "]")

	countStr := fmt.Sprintf("%d packages", len(m.filtered))
	countBadge := lipgloss.NewStyle().Foreground(subtleColor).Render(countStr)

	return lipgloss.JoinHorizontal(
		lipgloss.Center,
		title,
		"  ",
		filterTabs,
		"  ",
		sortBadge,
		"  ",
		countBadge,
	)
}

func (m *model) tableHeight() int {
	h := m.height - 7
	if h < 6 {
		return 6
	}
	return h
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.width > 50 {
			m.searchInput.Width = 32
		}
		m.initTable()
		return m, nil

	case brewFinishedMsg:
		m.showDetails = false
		m.modalAction = actionNone
		m.selectedItem = nil
		if msg.err != nil {
			m.statusMessage = fmt.Sprintf("Error: failed to %s %s: %v", msg.action, msg.itemName, msg.err)
			m.statusIsError = true
		} else {
			if msg.action == "upgrade" {
				m.statusMessage = fmt.Sprintf("✓ Successfully upgraded %s", msg.itemName)
			} else {
				m.statusMessage = fmt.Sprintf("✓ Successfully uninstalled %s", msg.itemName)
			}
			m.statusIsError = false
			newItems, err := loadPackages()
			if err == nil {
				m.allItems = newItems
				if msg.action == "uninstall" {
					m.searchInput.SetValue("")
					m.searching = false
				}
				m.applyFilters()
				m.initTable()
			} else {
				m.statusMessage += fmt.Sprintf(" (reload error: %v)", err)
			}
		}
		return m, nil

	case tea.MouseMsg:
		switch msg.Action {
		case tea.MouseActionPress:
			if msg.Button == tea.MouseButtonWheelUp {
				m.statusMessage = ""
				m.table.MoveUp(1)
				return m, nil
			} else if msg.Button == tea.MouseButtonWheelDown {
				m.statusMessage = ""
				m.table.MoveDown(1)
				return m, nil
			} else if msg.Button == tea.MouseButtonLeft {
				if m.showDetails {
					modalW := 68
					modalH := 18
					startX := (m.width - modalW) / 2
					endX := startX + modalW
					startY := (m.height - modalH) / 2
					endY := startY + modalH
					if msg.X < startX || msg.X > endX || msg.Y < startY || msg.Y > endY {
						if m.modalAction != actionNone {
							m.modalAction = actionNone
						} else {
							m.showDetails = false
						}
						return m, nil
					}
				} else if !m.searching && !m.showHelp {
					visibleRow := msg.Y - 5
					if visibleRow >= 0 && visibleRow < m.tableHeight() {
						lines := strings.Split(m.table.View(), "\n")
						if len(lines) > 2 {
							rendered := lines[2:]
							pre := strings.SplitN(tableSelectedStyle.Render("\x00"), "\x00", 2)[0]
							selIdxInView := -1
							if pre != "" {
								for i, rLine := range rendered {
									if strings.HasPrefix(rLine, pre) {
										selIdxInView = i
										break
									}
								}
							}
							if selIdxInView != -1 && visibleRow < len(rendered) {
								diff := visibleRow - selIdxInView
								if diff > 0 {
									m.table.MoveDown(diff)
								} else if diff < 0 {
									m.table.MoveUp(-diff)
								}
							}
							idx := m.table.Cursor()
							if idx >= 0 && idx < len(m.filtered) {
								it := m.filtered[idx]
								m.selectedItem = &it
								m.showDetails = true
								m.modalAction = actionNone
								m.statusMessage = ""
								return m, nil
							}
						}
					}
				}
			}
		}

	case tea.KeyMsg:
		if m.statusMessage != "" && !m.showDetails {
			m.statusMessage = ""
		}

		// When details modal is shown
		if m.showDetails {
			switch m.modalAction {
			case actionBlockedDelete:
				switch msg.String() {
				case "esc", "enter", "q", "x", "d":
					m.modalAction = actionNone
					m.showDetails = false
					m.selectedItem = nil
					return m, nil
				}
				return m, nil

			case actionConfirmDelete:
				switch msg.String() {
				case "x", "X", "d", "D":
					if m.selectedItem != nil {
						return m, runBrewCommand("uninstall", m.selectedItem)
					}
					m.modalAction = actionNone
					m.showDetails = false
					m.selectedItem = nil
					return m, nil
				case "n", "N", "esc", "q":
					m.modalAction = actionNone
					m.showDetails = false
					m.selectedItem = nil
					return m, nil
				}
				return m, nil
			case actionConfirmUpgrade:
				switch msg.String() {
				case "y", "Y":
					if m.selectedItem != nil {
						return m, runBrewCommand("upgrade", m.selectedItem)
					}
					m.modalAction = actionNone
					return m, nil
				case "n", "N", "esc", "q":
					m.modalAction = actionNone
					return m, nil
				}
				return m, nil

			default:
				switch msg.String() {
				case "esc", "enter", "q":
					m.showDetails = false
					m.modalAction = actionNone
					return m, nil
				case "o":
					if m.selectedItem != nil && m.selectedItem.Homepage != "" {
						_ = exec.Command("open", m.selectedItem.Homepage).Start()
					}
					return m, nil
				case "u":
					if m.selectedItem != nil {
						m.modalAction = actionConfirmUpgrade
					}
					return m, nil
				case "x", "d":
					if m.selectedItem != nil {
						if len(m.selectedItem.RequiredBy) > 0 {
							m.modalAction = actionBlockedDelete
						} else {
							m.modalAction = actionConfirmDelete
						}
					}
					return m, nil
				}
				return m, nil
			}
		}

		// When help modal is shown
		if m.showHelp {
			switch msg.String() {
			case "esc", "?", "enter", "q":
				m.showHelp = false
				return m, nil
			}
			return m, nil
		}

		// When live searching
		if m.searching {
			switch msg.String() {
			case "esc":
				m.searching = false
				m.searchInput.Blur()
				return m, nil
			case "enter":
				m.searching = false
				m.searchInput.Blur()
				return m, nil
			default:
				m.searchInput, cmd = m.searchInput.Update(msg)
				m.applyFilters()
				m.initTable()
				return m, cmd
			}
		}

		// Normal table navigation
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "/":
			m.statusMessage = ""
			m.searching = true
			m.searchInput.Focus()
			return m, textinput.Blink

		case "esc":
			m.statusMessage = ""
			if m.searchInput.Value() != "" {
				m.searchInput.SetValue("")
				m.applyFilters()
				m.initTable()
			}

		case "tab", "t":
			m.statusMessage = ""
			// Cycle filter: All -> Formula -> Cask -> All
			switch m.filterType {
			case FilterAll:
				m.filterType = FilterFormula
			case FilterFormula:
				m.filterType = FilterCask
			case FilterCask:
				m.filterType = FilterAll
			}
			m.applyFilters()
			m.initTable()

		case "s", "d":
			m.statusMessage = ""
			// Toggle sort: Date Desc <-> Date Asc
			if m.sortMode == SortDateDesc {
				m.sortMode = SortDateAsc
			} else {
				m.sortMode = SortDateDesc
			}
			m.applyFilters()
			m.initTable()

		case "n":
			m.statusMessage = ""
			if m.sortMode == SortNameAsc {
				m.sortMode = SortDateDesc
			} else {
				m.sortMode = SortNameAsc
			}
			m.applyFilters()
			m.initTable()

		case "enter":
			m.statusMessage = ""
			idx := m.table.Cursor()
			if idx >= 0 && idx < len(m.filtered) {
				it := m.filtered[idx]
				m.selectedItem = &it
				m.showDetails = true
				m.modalAction = actionNone
			}
			return m, nil

		case "x":
			m.statusMessage = ""
			idx := m.table.Cursor()
			if idx >= 0 && idx < len(m.filtered) {
				it := m.filtered[idx]
				m.selectedItem = &it
				m.showDetails = true
				if len(it.RequiredBy) > 0 {
					m.modalAction = actionBlockedDelete
				} else {
					m.modalAction = actionConfirmDelete
				}
			}
			return m, nil

		case "o":
			m.statusMessage = ""
			idx := m.table.Cursor()
			if idx >= 0 && idx < len(m.filtered) {
				it := m.filtered[idx]
				if it.Homepage != "" {
					_ = exec.Command("open", it.Homepage).Start()
				}
			}
			return m, nil

		case "?":
			m.statusMessage = ""
			m.showHelp = !m.showHelp
			return m, nil
		}
	}

	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m model) View() string {
	if m.showDetails && m.selectedItem != nil {
		return m.viewDetailsModal()
	}

	if m.showHelp {
		return m.viewHelpModal()
	}

	// 1. Header with title & filter indicator
	header := m.renderHeader()
	headerBlock := lipgloss.JoinVertical(lipgloss.Left, "", " "+header, "")
	// 2. Table view
	tableView := m.renderTableView()

	// 3. Bottom bar: search, status message, or shortcuts
	var bottomBar string
	if m.searching || m.searchInput.Value() != "" {
		searchLabel := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F59E0B")).Render("Search:")
		inputView := m.searchInput.View()
		tip := lipgloss.NewStyle().Foreground(subtleColor).Render(" (Esc to clear, Enter to return to table)")
		bottomBar = lipgloss.JoinHorizontal(lipgloss.Center, " ", searchLabel, " ", inputView, tip)
	} else if m.statusMessage != "" {
		color := lipgloss.Color("#22C55E")
		if m.statusIsError {
			color = lipgloss.Color("#EF4444")
		}
		statusBadge := lipgloss.NewStyle().Bold(true).Foreground(color).Render(" " + m.statusMessage)
		tip := lipgloss.NewStyle().Foreground(subtleColor).Render("  (press any key to dismiss)")
		bottomBar = lipgloss.JoinHorizontal(lipgloss.Center, statusBadge, tip)
	} else {
		shortcuts := "↑/↓: navigate • /: search • tab: filter • s: sort • enter: details • x: delete • ?: help • q: quit"
		bottomBar = statusStyle.Render(shortcuts)
	}

	return lipgloss.JoinVertical(lipgloss.Left, headerBlock, tableView, "", bottomBar)
}
func (m model) renderTableView() string {
	view := m.table.View()
	lines := strings.Split(view, "\n")
	if len(lines) <= 2 {
		return view
	}

	header := lines[:2]
	rendered := lines[2:]
	pre := strings.SplitN(tableSelectedStyle.Render("\x00"), "\x00", 2)[0]
	selIdxInView := -1
	if pre != "" {
		for i, rLine := range rendered {
			if strings.HasPrefix(rLine, pre) {
				selIdxInView = i
				break
			}
		}
	}

	result := make([]string, 0, len(lines))
	result = append(result, header...)

	for i, rLine := range rendered {
		if strings.TrimSpace(rLine) == "" {
			result = append(result, rLine)
			continue
		}
		if i == selIdxInView {
			result = append(result, rLine)
			continue
		}
		if selIdxInView != -1 {
			itemIdx := m.table.Cursor() + (i - selIdxInView)
			if itemIdx >= 0 && itemIdx < len(m.filtered) {
				if m.filtered[itemIdx].Type == "cask" {
					result = append(result, caskRowStyle.Render(rLine))
					continue
				}
			}
		}
		result = append(result, rLine)
	}

	return strings.Join(result, "\n")
}

func (m model) viewConfirmDeleteModal() string {
	it := m.selectedItem
	nameStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).Background(lipgloss.Color("#EF4444")).Padding(0, 1)
	title := nameStyle.Render("Uninstall " + it.Name)

	caskFlag := ""
	if it.Type == "cask" {
		caskFlag = "--cask "
	}
	cmdPreview := fmt.Sprintf("brew uninstall %s%s", caskFlag, it.Name)

	content := fmt.Sprintf(
		"%s\n\n"+
			"Are you sure you want to delete this package?\n\n"+
			"%s %s\n"+
			"%s %s\n\n"+
			"%s\n"+
			"  %s\n\n"+
			"%s",
		title,
		lipgloss.NewStyle().Bold(true).Render("Package:       "), it.Name,
		lipgloss.NewStyle().Bold(true).Render("Type:          "), strings.ToUpper(it.Type[:1])+it.Type[1:],
		lipgloss.NewStyle().Foreground(subtleColor).Render("Command to execute:"),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B")).Render(cmdPreview),
		lipgloss.NewStyle().Bold(true).Render("[x] Press x again to confirm delete  •  [n / Esc] Cancel"),
	)
	box := modalBoxStyle.Copy().BorderForeground(lipgloss.Color("#EF4444")).Render(content)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func (m model) viewBlockedDeleteModal() string {
	it := m.selectedItem
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).Background(lipgloss.Color("#F59E0B")).Padding(0, 1).Render("Cannot Delete " + it.Name)

	var depList strings.Builder
	maxShow := 8
	for i, dep := range it.RequiredBy {
		if i >= maxShow {
			depList.WriteString(fmt.Sprintf("  ... and %d more packages\n", len(it.RequiredBy)-maxShow))
			break
		}
		depList.WriteString(fmt.Sprintf("  • %s\n", dep))
	}

	content := fmt.Sprintf(
		"%s\n\n"+
			"%s is required by %d installed package(s):\n\n"+
			"%s\n"+
			"Uninstalling this formula would break these dependent packages.\n"+
			"Homebrew will refuse removal unless forced.\n\n"+
			"%s\n\n"+
			"%s",
		title,
		lipgloss.NewStyle().Bold(true).Render(it.Name),
		len(it.RequiredBy),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444")).Render(depList.String()),
		lipgloss.NewStyle().Foreground(subtleColor).Render("To remove this package, you must first uninstall the dependent packages."),
		lipgloss.NewStyle().Bold(true).Render("[Esc / Enter] Back to details"),
	)
	box := modalBoxStyle.Copy().BorderForeground(lipgloss.Color("#F59E0B")).Render(content)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func (m model) viewConfirmUpgradeModal() string {
	it := m.selectedItem
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).Background(lipgloss.Color("#2563EB")).Padding(0, 1).Render("Upgrade " + it.Name)

	caskFlag := ""
	if it.Type == "cask" {
		caskFlag = "--cask "
	}
	cmdPreview := fmt.Sprintf("brew upgrade %s%s", caskFlag, it.Name)

	content := fmt.Sprintf(
		"%s\n\n"+
			"Upgrade this package to the latest version?\n\n"+
			"%s %s\n"+
			"%s %s\n\n"+
			"%s\n"+
			"  %s\n\n"+
			"%s",
		title,
		lipgloss.NewStyle().Bold(true).Render("Package:        "), it.Name,
		lipgloss.NewStyle().Bold(true).Render("Current Version:"), it.Version,
		lipgloss.NewStyle().Foreground(subtleColor).Render("Command to execute:"),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#38BDF8")).Render(cmdPreview),
		lipgloss.NewStyle().Bold(true).Render("[y] Yes, upgrade  •  [n / Esc] Cancel"),
	)
	box := modalBoxStyle.Copy().BorderForeground(lipgloss.Color("#2563EB")).Render(content)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func (m model) viewDetailsModal() string {
	if m.modalAction == actionConfirmDelete {
		return m.viewConfirmDeleteModal()
	}
	if m.modalAction == actionBlockedDelete {
		return m.viewBlockedDeleteModal()
	}
	if m.modalAction == actionConfirmUpgrade {
		return m.viewConfirmUpgradeModal()
	}

	it := m.selectedItem
	typeBadge := badgeFormula.Render("Formula")
	if it.Type == "cask" {
		typeBadge = badgeCask.Render("Cask")
	}

	installedStr := it.InstalledAt.Format("Monday, Jan 02, 2006 at 15:04:05")
	if it.Epoch <= 0 {
		installedStr = "Unknown"
	}

	var depSection string
	if len(it.RequiredBy) > 0 {
		depSummary := strings.Join(it.RequiredBy, ", ")
		if len(it.RequiredBy) > 6 {
			depSummary = strings.Join(it.RequiredBy[:6], ", ") + fmt.Sprintf(" ... (+%d more)", len(it.RequiredBy)-6)
		}
		depSection = lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B")).Render(
			fmt.Sprintf("⚠️  Required by (%d): %s\n   (Cannot safely delete while dependents are installed)", len(it.RequiredBy), depSummary),
		)
	} else if it.Type == "formula" {
		depSection = lipgloss.NewStyle().Foreground(lipgloss.Color("#22C55E")).Render("✓  No other installed packages depend on this.")
	}

	actions := lipgloss.NewStyle().Bold(true).Render("Keys: [u] Upgrade • [x] Delete • [o] Homepage • [Esc/Enter] Close")

	content := fmt.Sprintf(
		"%s  %s\n\n"+
			"%s %s\n"+
			"%s %s\n"+
			"%s %s\n"+
			"%s %s\n\n"+
			"%s\n\n"+
			"%s\n\n"+
			"%s",
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).Background(accentColor).Padding(0, 1).Render(it.Name),
		typeBadge,
		lipgloss.NewStyle().Bold(true).Render("Version:        "), it.Version,
		lipgloss.NewStyle().Bold(true).Render("First Installed:"), installedStr,
		lipgloss.NewStyle().Bold(true).Render("Homepage:       "), it.Homepage,
		lipgloss.NewStyle().Bold(true).Render("Description:    "), it.Desc,
		depSection,
		actions,
		"",
	)

	box := modalBoxStyle.Render(content)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func (m model) viewHelpModal() string {
	helpText := `Keyboard & Mouse Shortcuts

  Navigation & Mouse
    ↑ / k       Move selection up
    ↓ / j       Move selection down
    Wheel Up    Scroll table up
    Wheel Down  Scroll table down
    Click row   Open package details & actions
    Home / g    Jump to top
    End / G     Jump to bottom

  Filtering & Search
    /           Search packages (live filter by name & desc)
    Tab / t     Cycle view: All → Formulae → Casks
    Esc         Clear active search

  Sorting
    s / d       Toggle sort: Newest first ↔ Oldest first
    n           Sort alphabetically (A-Z)

  Actions (in Details Modal)
    Enter       Open details for selected package
    u           Upgrade package (runs brew upgrade)
    x / d       Delete package (press x a second time to confirm)
    o           Open package homepage in browser
    ?           Toggle this help overlay
    q / Ctrl+C  Quit brew-ls`

	box := modalBoxStyle.Render(helpText)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

// Print plain non-interactive table
func printPlain(items []PackageItem, filter FilterType, reverse bool, asJSON bool) {
	var filtered []PackageItem
	for _, it := range items {
		if filter == FilterFormula && it.Type != "formula" {
			continue
		}
		if filter == FilterCask && it.Type != "cask" {
			continue
		}
		filtered = append(filtered, it)
	}

	if reverse {
		sort.SliceStable(filtered, func(i, j int) bool {
			if filtered[i].Epoch != filtered[j].Epoch {
				return filtered[i].Epoch < filtered[j].Epoch
			}
			return strings.ToLower(filtered[i].Name) < strings.ToLower(filtered[j].Name)
		})
	} else {
		sort.SliceStable(filtered, func(i, j int) bool {
			if filtered[i].Epoch != filtered[j].Epoch {
				return filtered[i].Epoch > filtered[j].Epoch
			}
			return strings.ToLower(filtered[i].Name) < strings.ToLower(filtered[j].Name)
		})
	}

	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(filtered)
		return
	}

	maxName := 4
	maxVer := 7
	for _, it := range filtered {
		if len(it.Name) > maxName {
			maxName = len(it.Name)
		}
		if len(it.Version) > maxVer {
			maxVer = len(it.Version)
		}
	}

	fmt.Printf("%-19s  %-7s  %-*s  %-*s  %s\n",
		"FIRST INSTALLED", "TYPE", maxName, "NAME", maxVer, "VERSION", "DESCRIPTION")

	isTTY := term.IsTerminal(int(os.Stdout.Fd()))
	for _, it := range filtered {
		dateStr := it.InstalledAt.Format("2006-01-02 15:04:05")
		if it.Epoch <= 0 {
			dateStr = "unknown"
		}
		line := fmt.Sprintf("%-19s  %-7s  %-*s  %-*s  %s",
			dateStr, it.Type, maxName, it.Name, maxVer, it.Version, it.Desc)
		if it.Type == "cask" && isTTY {
			line = caskRowStyle.Render(line)
		}
		fmt.Println(line)
	}
}

func main() {
	var (
		flagFormula = flag.Bool("formula", false, "Show only formulae")
		flagF       = flag.Bool("f", false, "Show only formulae (shorthand)")
		flagCask    = flag.Bool("cask", false, "Show only casks")
		flagC       = flag.Bool("c", false, "Show only casks (shorthand)")
		flagAll     = flag.Bool("all", false, "Show all packages")
		flagA       = flag.Bool("a", false, "Show all packages (shorthand)")
		flagPlain   = flag.Bool("plain", false, "Output plain text table without TUI")
		flagJSON    = flag.Bool("json", false, "Output JSON without TUI")
		flagReverse = flag.Bool("reverse", false, "Sort oldest first (in plain mode)")
		flagR       = flag.Bool("r", false, "Sort oldest first (shorthand)")
	)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: brew-ls [OPTIONS]\n\nInteractive TUI table of manually installed Homebrew packages with initial install date.\n\nOptions:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	isTTY := term.IsTerminal(int(os.Stdout.Fd()))

	// Determine initial filter
	initialFilter := FilterAll
	if *flagFormula || *flagF {
		initialFilter = FilterFormula
	} else if *flagCask || *flagC {
		initialFilter = FilterCask
	} else if *flagAll || *flagA {
		initialFilter = FilterAll
	}

	// Load packages
	items, err := loadPackages()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading packages: %v\n", err)
		os.Exit(1)
	}

	// Plain / Scripted mode
	if *flagPlain || *flagJSON || !isTTY {
		printPlain(items, initialFilter, *flagReverse || *flagR, *flagJSON)
		return
	}

	// Launch Bubble Tea Interactive TUI
	p := tea.NewProgram(
		initialModel(items, initialFilter),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}
}
