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
	Type        string    `json:"type"` // "formula" or "cask"
	Version     string    `json:"version"`
	Desc        string    `json:"desc"`
	Homepage    string    `json:"homepage"`
	InstalledAt time.Time `json:"installed_at"`
	Epoch       int64     `json:"epoch"`
}

type BrewJSON struct {
	Formulae []Formula `json:"formulae"`
	Casks    []Cask    `json:"casks"`
}

type Formula struct {
	Name      string      `json:"name"`
	FullName  string      `json:"full_name"`
	Desc      string      `json:"desc"`
	Homepage  string      `json:"homepage"`
	Installed []Installed `json:"installed"`
}

type Installed struct {
	Version            string `json:"version"`
	Time               int64  `json:"time"`
	InstalledOnRequest bool   `json:"installed_on_request"`
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

		items = append(items, PackageItem{
			Name:        f.Name,
			Type:        "formula",
			Version:     lastInst.Version,
			Desc:        f.Desc,
			Homepage:    f.Homepage,
			InstalledAt: time.Unix(epoch, 0),
			Epoch:       epoch,
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
)

// Bubble Tea Model
type model struct {
	allItems     []PackageItem
	filtered     []PackageItem
	table        table.Model
	searchInput  textinput.Model
	searching    bool
	filterType   FilterType
	sortMode     SortMode
	width        int
	height       int
	showDetails  bool
	selectedItem *PackageItem
	showHelp     bool
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
		sort.Slice(filtered, func(i, j int) bool {
			return filtered[i].Epoch > filtered[j].Epoch
		})
	case SortDateAsc:
		sort.Slice(filtered, func(i, j int) bool {
			return filtered[i].Epoch < filtered[j].Epoch
		})
	case SortNameAsc:
		sort.Slice(filtered, func(i, j int) bool {
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
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(selectedColor).
		Bold(true)
	t.SetStyles(s)

	m.table = t
}

func (m *model) tableHeight() int {
	h := m.height - 7 // Room for title, tabs, status bar, and search
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

	case tea.KeyMsg:
		// When details modal is shown
		if m.showDetails {
			switch msg.String() {
			case "esc", "enter", "q":
				m.showDetails = false
				return m, nil
			case "o":
				if m.selectedItem != nil && m.selectedItem.Homepage != "" {
					_ = exec.Command("open", m.selectedItem.Homepage).Start()
				}
				return m, nil
			}
			return m, nil
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
			m.searching = true
			m.searchInput.Focus()
			return m, textinput.Blink

		case "esc":
			if m.searchInput.Value() != "" {
				m.searchInput.SetValue("")
				m.applyFilters()
				m.initTable()
			}

		case "tab", "t":
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
			// Toggle sort: Date Desc <-> Date Asc
			if m.sortMode == SortDateDesc {
				m.sortMode = SortDateAsc
			} else {
				m.sortMode = SortDateDesc
			}
			m.applyFilters()
			m.initTable()

		case "n":
			if m.sortMode == SortNameAsc {
				m.sortMode = SortDateDesc
			} else {
				m.sortMode = SortNameAsc
			}
			m.applyFilters()
			m.initTable()

		case "enter":
			idx := m.table.Cursor()
			if idx >= 0 && idx < len(m.filtered) {
				it := m.filtered[idx]
				m.selectedItem = &it
				m.showDetails = true
			}
			return m, nil

		case "o":
			idx := m.table.Cursor()
			if idx >= 0 && idx < len(m.filtered) {
				it := m.filtered[idx]
				if it.Homepage != "" {
					_ = exec.Command("open", it.Homepage).Start()
				}
			}
			return m, nil

		case "?":
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

	header := lipgloss.JoinHorizontal(
		lipgloss.Center,
		title,
		"  ",
		filterTabs,
		"  ",
		sortBadge,
		"  ",
		countBadge,
	)

	// 2. Table view
	tableView := m.table.View()

	// 3. Bottom bar: search or shortcuts
	var bottomBar string
	if m.searching || m.searchInput.Value() != "" {
		searchLabel := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F59E0B")).Render("Search:")
		inputView := m.searchInput.View()
		tip := lipgloss.NewStyle().Foreground(subtleColor).Render(" (Esc to clear, Enter to return to table)")
		bottomBar = lipgloss.JoinHorizontal(lipgloss.Center, " ", searchLabel, " ", inputView, tip)
	} else {
		shortcuts := "↑/↓: navigate • /: search • tab: filter • s: sort date • n: sort name • enter: details • o: open homepage • ?: help • q: quit"
		bottomBar = statusStyle.Render(shortcuts)
	}

	return lipgloss.JoinVertical(lipgloss.Left, "\n", " "+header, "\n", tableView, "\n", bottomBar)
}

func (m model) viewDetailsModal() string {
	it := m.selectedItem
	typeBadge := badgeFormula.Render("Formula")
	if it.Type == "cask" {
		typeBadge = badgeCask.Render("Cask")
	}

	installedStr := it.InstalledAt.Format("Monday, Jan 02, 2006 at 15:04:05")
	if it.Epoch <= 0 {
		installedStr = "Unknown"
	}

	content := fmt.Sprintf(
		"%s  %s\n\n"+
			"%s %s\n"+
			"%s %s\n"+
			"%s %s\n"+
			"%s %s\n\n"+
			"%s\n\n"+
			"%s",
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).Background(accentColor).Padding(0, 1).Render(it.Name),
		typeBadge,
		lipgloss.NewStyle().Bold(true).Render("Version:        "), it.Version,
		lipgloss.NewStyle().Bold(true).Render("First Installed:"), installedStr,
		lipgloss.NewStyle().Bold(true).Render("Homepage:       "), it.Homepage,
		lipgloss.NewStyle().Bold(true).Render("Description:    "), it.Desc,
		lipgloss.NewStyle().Foreground(subtleColor).Render("Keys: [o] Open homepage in browser • [Esc/Enter] Close"),
		"",
	)

	box := modalBoxStyle.Render(content)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func (m model) viewHelpModal() string {
	helpText := `Keyboard Shortcuts

  Navigation
    ↑ / k       Move selection up
    ↓ / j       Move selection down
    Home / g    Jump to top
    End / G     Jump to bottom

  Filtering & Search
    /           Search packages (live filter by name & desc)
    Tab / t     Cycle view: All → Formulae → Casks
    Esc         Clear active search

  Sorting
    s / d       Toggle sort: Newest first ↔ Oldest first
    n           Sort alphabetically (A-Z)

  Actions
    Enter       View full package details
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
		sort.Slice(filtered, func(i, j int) bool {
			return filtered[i].Epoch < filtered[j].Epoch
		})
	} else {
		sort.Slice(filtered, func(i, j int) bool {
			return filtered[i].Epoch > filtered[j].Epoch
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

	for _, it := range filtered {
		dateStr := it.InstalledAt.Format("2006-01-02 15:04:05")
		if it.Epoch <= 0 {
			dateStr = "unknown"
		}
		fmt.Printf("%-19s  %-7s  %-*s  %-*s  %s\n",
			dateStr, it.Type, maxName, it.Name, maxVer, it.Version, it.Desc)
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
