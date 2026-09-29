package ui

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/dustinmichels/repo-ls/git"
	"github.com/dustinmichels/repo-ls/models"
)

// Messages
type tokeiResultMsg struct {
	path  string
	stats *models.TokeiStats
	err   error
}

type ghEnrichMsg struct {
	repos []*models.Repo
}

type statusClearMsg struct{}
// TableItemKind represents whether an item is a repository or a folder group header.
type TableItemKind int

const (
	ItemKindRepo TableItemKind = iota
	ItemKindFolder
)

// TableItem is a single row in the explorer table.
type TableItem struct {
	Kind        TableItemKind
	Repo        *models.Repo
	Folder      string
	RepoCount   int
	DirtyCount  int
	HasChildren bool
	IsExpanded  bool
	IsChild     bool
}

// Model is the Bubble Tea model for repo-ls.
type Model struct {
	rootDir            string
	allRepos           []*models.Repo
	filtered           []*models.Repo
	items              []TableItem
	collapsedFolders   map[string]bool
	noSelection        bool
	cursor             int
	scrollOffset       int
	width              int
	height             int
	filterMode         models.FilterMode
	sortMode           models.SortMode
	searchInput        textinput.Model
	searching          bool
	tokeiCache         map[string]*models.TokeiStats
	loadingTokei       map[string]bool
	showInspectorFull  bool
	inspectorScroll    int
	showDeleteModal    bool
	deleteStep         int // 0: choose option (1=local, 2=github, 3=both), 1: confirm name input
	deleteChoice       int // 1=local, 2=github, 3=both
	deleteConfirmInput textinput.Model
	statusMsg          string
	statusIsError      bool
}

// NewModel initializes the TUI model.
func NewModel(rootDir string, repos []*models.Repo) Model {
	ti := textinput.New()
	ti.Placeholder = "Filter by name, path, or remote..."
	ti.Prompt = " / "
	ti.PromptStyle = searchPromptStyle
	ti.CharLimit = 64
	ti.Width = 36

	delInput := textinput.New()
	delInput.Placeholder = "Type repo name to confirm..."
	delInput.Prompt = "> "
	delInput.PromptStyle = dangerStyle
	delInput.CharLimit = 64
	delInput.Width = 30

	m := Model{
		rootDir:            rootDir,
		allRepos:           repos,
		tokeiCache:         make(map[string]*models.TokeiStats),
		loadingTokei:       make(map[string]bool),
		collapsedFolders:   make(map[string]bool),
		searchInput:        ti,
		deleteConfirmInput: delInput,
		filterMode:         models.FilterAll,
		sortMode:           models.SortPath,
		width:              110,
		height:             30,
	}

	m.applyFiltersAndSort()
	return m
}

func (m *Model) currentItem() *TableItem {
	if m.noSelection || m.cursor < 0 || m.cursor >= len(m.items) {
		return nil
	}
	return &m.items[m.cursor]
}

func (m *Model) currentRepo() *models.Repo {
	if item := m.currentItem(); item != nil && item.Kind == ItemKindRepo {
		return item.Repo
	}
	if len(m.items) == 0 && !m.noSelection && m.cursor >= 0 && m.cursor < len(m.filtered) {
		return m.filtered[m.cursor]
	}
	return nil
}

func padRight(s string, w int) string {
	sw := lipgloss.Width(s)
	if sw > w {
		return ansi.Truncate(s, w, "…")
	}
	return s + strings.Repeat(" ", w-sw)
}

func (m *Model) deselect() tea.Cmd {
	m.noSelection = true
	m.cursor = -1
	m.adjustScroll()
	return m.loadAllTokeiCmd()
}

func (m *Model) selectRow(idx int) {
	m.noSelection = false
	m.cursor = idx
	m.adjustScroll()
}

func (m *Model) applyFiltersAndSort() {
	query := strings.ToLower(strings.TrimSpace(m.searchInput.Value()))

	// Record current selection key before rebuild to prevent drift
	var selectedKey string
	if !m.noSelection && m.cursor >= 0 && m.cursor < len(m.items) {
		it := m.items[m.cursor]
		if it.Kind == ItemKindFolder {
			selectedKey = "dir:" + it.Folder
		} else if it.Repo != nil {
			selectedKey = "repo:" + it.Repo.Path
		}
	}

	var filtered []*models.Repo
	for _, r := range m.allRepos {
		// 1. Filter mode
		matchFilter := true
		switch m.filterMode {
		case models.FilterUncommitted:
			matchFilter = r.HasUncommitted
		case models.FilterClean:
			matchFilter = !r.HasUncommitted
		case models.FilterGitHub:
			matchFilter = r.IsGitHub
		case models.FilterLocal:
			matchFilter = !r.IsGitHub
		case models.FilterPublic:
			matchFilter = r.GitHubVisibility == models.VisibilityPublic
		case models.FilterPrivate:
			matchFilter = r.GitHubVisibility == models.VisibilityPrivate
		}

		if !matchFilter {
			continue
		}

		// 2. Query search
		if query != "" {
			nameMatch := strings.Contains(strings.ToLower(r.Name), query)
			pathMatch := strings.Contains(strings.ToLower(r.RelPath), query)
			remoteMatch := strings.Contains(strings.ToLower(r.RemoteURL), query) || strings.Contains(strings.ToLower(r.GitHubSlug), query)
			if !nameMatch && !pathMatch && !remoteMatch {
				continue
			}
		}

		filtered = append(filtered, r)
	}

	// 3. Sort
	switch m.sortMode {
	case models.SortName:
		sort.Slice(filtered, func(i, j int) bool {
			return strings.ToLower(filtered[i].Name) < strings.ToLower(filtered[j].Name)
		})
	case models.SortModified:
		sort.Slice(filtered, func(i, j int) bool {
			timeI := filtered[i].LastModified
			if timeI.IsZero() && filtered[i].LastCommit != nil {
				timeI = filtered[i].LastCommit.Date
			}
			timeJ := filtered[j].LastModified
			if timeJ.IsZero() && filtered[j].LastCommit != nil {
				timeJ = filtered[j].LastCommit.Date
			}
			if !timeI.Equal(timeJ) {
				return timeI.After(timeJ)
			}
			return filtered[i].RelPath < filtered[j].RelPath
		})
	case models.SortCreated:
		sort.Slice(filtered, func(i, j int) bool {
			if !filtered[i].DateCreated.Equal(filtered[j].DateCreated) {
				return filtered[i].DateCreated.After(filtered[j].DateCreated)
			}
			return filtered[i].RelPath < filtered[j].RelPath
		})
	case models.SortDirtyFirst:
		sort.Slice(filtered, func(i, j int) bool {
			if filtered[i].HasUncommitted != filtered[j].HasUncommitted {
				return filtered[i].HasUncommitted
			}
			return filtered[i].RelPath < filtered[j].RelPath
		})
	default: // SortPath
		sort.Slice(filtered, func(i, j int) bool {
			return filtered[i].RelPath < filtered[j].RelPath
		})
	}

	m.filtered = filtered

	// 4. Build table items (with folder & parent-repo grouping)
	rootRepoMap := make(map[string]*models.Repo)
	nestedByFolder := make(map[string][]*models.Repo)
	var folderOrder []string
	seenFolder := make(map[string]bool)

	for _, r := range filtered {
		slashPath := filepath.ToSlash(r.RelPath)
		if !strings.Contains(slashPath, "/") {
			rootRepoMap[r.RelPath] = r
		} else {
			folder := strings.Split(slashPath, "/")[0]
			if !seenFolder[folder] {
				seenFolder[folder] = true
				folderOrder = append(folderOrder, folder)
			}
			nestedByFolder[folder] = append(nestedByFolder[folder], r)
		}
	}

	type topLevelItem struct {
		key      string
		isFolder bool
		repo     *models.Repo
		children []*models.Repo
	}

	var topList []topLevelItem
	// Pure folders (folders with no root repo)
	for _, f := range folderOrder {
		if _, hasRoot := rootRepoMap[f]; !hasRoot {
			topList = append(topList, topLevelItem{
				key:      f,
				isFolder: true,
				children: nestedByFolder[f],
			})
		}
	}
	// Root repos (with any nested repos associated)
	for _, r := range filtered {
		slashPath := filepath.ToSlash(r.RelPath)
		if !strings.Contains(slashPath, "/") {
			topList = append(topList, topLevelItem{
				key:      r.RelPath,
				isFolder: false,
				repo:     r,
				children: nestedByFolder[r.RelPath],
			})
		}
	}

	if m.sortMode == models.SortPath {
		sort.Slice(topList, func(i, j int) bool {
			return topList[i].key < topList[j].key
		})
	}

	var items []TableItem
	for _, it := range topList {
		isExp := !m.collapsedFolders[it.key] || query != ""
		if it.isFolder {
			dirtyCount := 0
			for _, c := range it.children {
				if c.HasUncommitted {
					dirtyCount++
				}
			}
			items = append(items, TableItem{
				Kind:       ItemKindFolder,
				Folder:     it.key,
				RepoCount:  len(it.children),
				DirtyCount: dirtyCount,
				IsExpanded: isExp,
			})
			if isExp {
				for _, c := range it.children {
					items = append(items, TableItem{
						Kind:    ItemKindRepo,
						Repo:    c,
						IsChild: true,
					})
				}
			}
		} else {
			hasChildren := len(it.children) > 0
			items = append(items, TableItem{
				Kind:        ItemKindRepo,
				Repo:        it.repo,
				HasChildren: hasChildren,
				RepoCount:   len(it.children),
				IsExpanded:  isExp,
			})
			if hasChildren && isExp {
				for _, c := range it.children {
					items = append(items, TableItem{
						Kind:    ItemKindRepo,
						Repo:    c,
						IsChild: true,
					})
				}
			}
		}
	}

	m.items = items

	// Restore selection by key, falling back to bounds clamp
	if m.noSelection || len(m.items) == 0 {
		m.noSelection = true
		m.cursor = -1
	} else if selectedKey != "" {
		found := -1
		for i, it := range m.items {
			key := ""
			if it.Kind == ItemKindFolder {
				key = "dir:" + it.Folder
			} else if it.Repo != nil {
				key = "repo:" + it.Repo.Path
			}
			if key == selectedKey {
				found = i
				break
			}
		}
		if found >= 0 {
			m.selectRow(found)
		} else if len(m.items) > 0 {
			if m.cursor >= len(m.items) {
				m.selectRow(len(m.items) - 1)
			} else if m.cursor < 0 {
				m.selectRow(0)
			} else {
				m.selectRow(m.cursor)
			}
		} else {
			m.noSelection = true
			m.cursor = -1
		}
	} else {
		if len(m.items) > 0 {
			m.selectRow(0)
		} else {
			m.noSelection = true
			m.cursor = -1
		}
	}
	m.adjustScroll()
}

func (m *Model) adjustScroll() {
	tableH := m.tableBodyHeight()
	if tableH <= 0 {
		tableH = 10
	}

	if m.cursor < 0 {
		m.scrollOffset = 0
		return
	}

	if m.cursor < m.scrollOffset {
		m.scrollOffset = m.cursor
	} else if m.cursor >= m.scrollOffset+tableH {
		m.scrollOffset = m.cursor - tableH + 1
	}
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
}

func (m *Model) tableBodyHeight() int {
	// Total height minus headers, filter bar, help bar, status line
	headerH := 5
	if m.searching || m.searchInput.Value() != "" {
		headerH = 6
	}
	footerH := 2
	h := m.height - headerH - footerH
	if h < 5 {
		return 5
	}
	return h
}

func (m Model) Init() tea.Cmd {
	var cmds []tea.Cmd

	// Trigger initial Tokei load for first repo if any
	if cur := m.currentRepo(); cur != nil {
		cmds = append(cmds, m.loadTokeiCmd(cur.Path))
	}

	// Trigger GitHub visibility enrichment in background
	cmds = append(cmds, func() tea.Msg {
		git.EnrichGitHubVisibility(m.allRepos)
		return ghEnrichMsg{repos: m.allRepos}
	})

	return tea.Batch(cmds...)
}

func (m *Model) loadTokeiCmd(path string) tea.Cmd {
	if _, ok := m.tokeiCache[path]; ok {
		return nil
	}
	if m.loadingTokei[path] {
		return nil
	}
	m.loadingTokei[path] = true

	return func() tea.Msg {
		stats, err := git.RunTokei(path)
		return tokeiResultMsg{
			path:  path,
			stats: stats,
			err:   err,
		}
	}
}
func dedupeNestedPaths(paths []string) []string {
	if len(paths) <= 1 {
		return paths
	}
	sorted := make([]string, len(paths))
	copy(sorted, paths)
	sort.Strings(sorted)

	var result []string
	for _, p := range sorted {
		if len(result) > 0 {
			prev := result[len(result)-1]
			if strings.HasPrefix(p, prev+"/") {
				continue
			}
		}
		result = append(result, p)
	}
	return result
}

const allReposTokeiKey = "__all_repos__"

func (m *Model) loadAllTokeiCmd() tea.Cmd {
	if _, ok := m.tokeiCache[allReposTokeiKey]; ok {
		return nil
	}
	if m.loadingTokei[allReposTokeiKey] {
		return nil
	}
	m.loadingTokei[allReposTokeiKey] = true

	var paths []string
	for _, r := range m.allRepos {
		paths = append(paths, r.Path)
	}

	return func() tea.Msg {
		stats, err := git.RunTokei(dedupeNestedPaths(paths)...)
		return tokeiResultMsg{
			path:  allReposTokeiKey,
			stats: stats,
			err:   err,
		}
	}
}

func (m *Model) loadFolderTokeiCmd(folder string) tea.Cmd {
	key := "__folder__" + folder
	if _, ok := m.tokeiCache[key]; ok {
		return nil
	}
	if m.loadingTokei[key] {
		return nil
	}
	m.loadingTokei[key] = true

	var paths []string
	for _, r := range m.allRepos {
		slashPath := filepath.ToSlash(r.RelPath)
		if slashPath == folder || strings.HasPrefix(slashPath, folder+"/") {
			paths = append(paths, r.Path)
		}
	}

	return func() tea.Msg {
		stats, err := git.RunTokei(dedupeNestedPaths(paths)...)
		return tokeiResultMsg{
			path:  key,
			stats: stats,
			err:   err,
		}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.adjustScroll()
		return m, nil

	case tokeiResultMsg:
		m.loadingTokei[msg.path] = false
		m.tokeiCache[msg.path] = msg.stats
		for _, r := range m.allRepos {
			if r.Path == msg.path {
				r.Tokei = msg.stats
				break
			}
		}
		return m, nil

	case ghEnrichMsg:
		m.applyFiltersAndSort()
		return m, nil

	case statusClearMsg:
		m.statusMsg = ""
		m.statusIsError = false
		return m, nil

	case tea.MouseMsg:
		cmd := m.handleMouse(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)

	case tea.KeyMsg:
		// Delete modal keyboard handling
		if m.showDeleteModal {
			return m.handleDeleteKeys(msg)
		}

		// Search input handling
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
			case "tab":
				m.searching = false
				m.searchInput.Blur()
				filterOrder := []models.FilterMode{
					models.FilterAll,
					models.FilterUncommitted,
					models.FilterClean,
					models.FilterGitHub,
					models.FilterLocal,
					models.FilterPublic,
					models.FilterPrivate,
				}
				currIdx := 0
				for idx, f := range filterOrder {
					if f == m.filterMode {
						currIdx = idx
						break
					}
				}
				m.filterMode = filterOrder[(currIdx+1)%len(filterOrder)]
				m.applyFiltersAndSort()
				return m, nil
			default:
				var tiCmd tea.Cmd
				m.searchInput, tiCmd = m.searchInput.Update(msg)
				m.applyFiltersAndSort()
				cur := m.currentRepo()
				if cur != nil {
					cmds = append(cmds, m.loadTokeiCmd(cur.Path))
				}
				return m, tiCmd
			}
		}

		// Inspector full view keyboard handling
		if m.showInspectorFull {
			switch msg.String() {
			case "esc", "q", "enter", "i", "backspace":
				m.showInspectorFull = false
				return m, nil
			case "up", "k":
				if m.inspectorScroll > 0 {
					m.inspectorScroll--
				}
				return m, nil
			case "down", "j":
				m.inspectorScroll++
				return m, nil
			case "o":
				if cur := m.currentRepo(); cur != nil && cur.GitHubWebURL != "" {
					_ = openURL(cur.GitHubWebURL)
				}
				return m, nil
			case "d", "x":
				m.showInspectorFull = false
				m.openDeleteModal()
				return m, nil
			}
			return m, nil
		}

		// Main navigation keys
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				m.adjustScroll()
				if cur := m.currentRepo(); cur != nil {
					cmds = append(cmds, m.loadTokeiCmd(cur.Path))
				} else if item := m.currentItem(); item != nil && item.Kind == ItemKindFolder {
					cmds = append(cmds, m.loadFolderTokeiCmd(item.Folder))
				}
			} else if m.cursor == 0 {
				m.noSelection = true
				m.cursor = -1
				m.adjustScroll()
				cmds = append(cmds, m.loadAllTokeiCmd())
			}

		case "down", "j":
			if (m.noSelection || m.cursor == -1) && len(m.items) > 0 {
				m.noSelection = false
				m.cursor = 0
				m.adjustScroll()
				if cur := m.currentRepo(); cur != nil {
					cmds = append(cmds, m.loadTokeiCmd(cur.Path))
				} else if item := m.currentItem(); item != nil && item.Kind == ItemKindFolder {
					cmds = append(cmds, m.loadFolderTokeiCmd(item.Folder))
				}
			} else if m.cursor < len(m.items)-1 {
				m.noSelection = false
				m.cursor++
				m.adjustScroll()
				if cur := m.currentRepo(); cur != nil {
					cmds = append(cmds, m.loadTokeiCmd(cur.Path))
				} else if item := m.currentItem(); item != nil && item.Kind == ItemKindFolder {
					cmds = append(cmds, m.loadFolderTokeiCmd(item.Folder))
				}
			}

		case "home":
			if (m.cursor == 0 && !m.noSelection) || m.noSelection {
				m.noSelection = true
				m.cursor = -1
				m.adjustScroll()
				cmds = append(cmds, m.loadAllTokeiCmd())
			} else {
				m.noSelection = false
				m.cursor = 0
				m.adjustScroll()
				if cur := m.currentRepo(); cur != nil {
					cmds = append(cmds, m.loadTokeiCmd(cur.Path))
				}
			}

		case "end", "G":
			if len(m.items) > 0 {
				m.noSelection = false
				m.cursor = len(m.items) - 1
				m.adjustScroll()
				if cur := m.currentRepo(); cur != nil {
					cmds = append(cmds, m.loadTokeiCmd(cur.Path))
				} else if item := m.currentItem(); item != nil && item.Kind == ItemKindFolder {
					cmds = append(cmds, m.loadFolderTokeiCmd(item.Folder))
				}
			}

		case "pgup":
			page := m.tableBodyHeight()
			m.cursor -= page
			if m.cursor < 0 {
				m.noSelection = true
				m.cursor = -1
				m.adjustScroll()
				cmds = append(cmds, m.loadAllTokeiCmd())
			} else {
				m.noSelection = false
				m.adjustScroll()
				if cur := m.currentRepo(); cur != nil {
					cmds = append(cmds, m.loadTokeiCmd(cur.Path))
				} else if item := m.currentItem(); item != nil && item.Kind == ItemKindFolder {
					cmds = append(cmds, m.loadFolderTokeiCmd(item.Folder))
				}
			}

		case "pgdown":
			if len(m.items) > 0 {
				m.noSelection = false
				page := m.tableBodyHeight()
				if m.cursor < 0 {
					m.cursor = 0
				} else {
					m.cursor += page
				}
				if m.cursor >= len(m.items) {
					m.cursor = len(m.items) - 1
				}
				m.adjustScroll()
				if cur := m.currentRepo(); cur != nil {
					cmds = append(cmds, m.loadTokeiCmd(cur.Path))
				} else if item := m.currentItem(); item != nil && item.Kind == ItemKindFolder {
					cmds = append(cmds, m.loadFolderTokeiCmd(item.Folder))
				}
			}
		case "enter":
			if item := m.currentItem(); item != nil && item.Kind == ItemKindFolder {
				m.collapsedFolders[item.Folder] = !m.collapsedFolders[item.Folder]
				m.applyFiltersAndSort()
				return m, nil
			}
			if len(m.items) > 0 || m.cursor == -1 {
				m.showInspectorFull = true
				m.inspectorScroll = 0
			}

		case "i":
			if len(m.items) > 0 || m.cursor == -1 {
				m.showInspectorFull = true
				m.inspectorScroll = 0
			}

		case " ", "left", "right":
			item := m.currentItem()
			if item != nil {
				if item.Kind == ItemKindFolder {
					m.collapsedFolders[item.Folder] = !m.collapsedFolders[item.Folder]
					m.applyFiltersAndSort()
					return m, nil
				}
				if item.HasChildren && item.Repo != nil {
					folder := strings.Split(filepath.ToSlash(item.Repo.RelPath), "/")[0]
					m.collapsedFolders[folder] = !m.collapsedFolders[folder]
					m.applyFiltersAndSort()
					return m, nil
				}
			}
		case "c":
			for _, r := range m.allRepos {
				if strings.Contains(filepath.ToSlash(r.RelPath), "/") {
					f := strings.Split(filepath.ToSlash(r.RelPath), "/")[0]
					m.collapsedFolders[f] = true
				}
			}
			m.applyFiltersAndSort()

		case "e":
			m.collapsedFolders = make(map[string]bool)
			m.applyFiltersAndSort()

		case "/":
			m.searching = true
			m.searchInput.Focus()
			return m, textinput.Blink

		case "esc":
			if m.searchInput.Value() != "" {
				m.searchInput.SetValue("")
				m.applyFiltersAndSort()
				if cur := m.currentRepo(); cur != nil {
					cmds = append(cmds, m.loadTokeiCmd(cur.Path))
				}
			} else if !m.noSelection {
				m.noSelection = true
				m.cursor = -1
				m.adjustScroll()
				cmds = append(cmds, m.loadAllTokeiCmd())
			}

		case "tab":
			filterOrder := []models.FilterMode{
				models.FilterAll,
				models.FilterUncommitted,
				models.FilterClean,
				models.FilterGitHub,
				models.FilterLocal,
				models.FilterPublic,
				models.FilterPrivate,
			}
			currIdx := 0
			for idx, f := range filterOrder {
				if f == m.filterMode {
					currIdx = idx
					break
				}
			}
			m.filterMode = filterOrder[(currIdx+1)%len(filterOrder)]
			m.applyFiltersAndSort()
			if m.cursor == -1 {
				cmds = append(cmds, m.loadAllTokeiCmd())
			} else if cur := m.currentRepo(); cur != nil {
				cmds = append(cmds, m.loadTokeiCmd(cur.Path))
			}

		case "shift+tab", "backtab":
			filterOrder := []models.FilterMode{
				models.FilterAll,
				models.FilterUncommitted,
				models.FilterClean,
				models.FilterGitHub,
				models.FilterLocal,
				models.FilterPublic,
				models.FilterPrivate,
			}
			currIdx := 0
			for idx, f := range filterOrder {
				if f == m.filterMode {
					currIdx = idx
					break
				}
			}
			m.filterMode = filterOrder[(currIdx-1+len(filterOrder))%len(filterOrder)]
			m.applyFiltersAndSort()
			if m.cursor == -1 {
				cmds = append(cmds, m.loadAllTokeiCmd())
			} else if cur := m.currentRepo(); cur != nil {
				cmds = append(cmds, m.loadTokeiCmd(cur.Path))
			}
		// Filters
		case "a":
			m.filterMode = models.FilterAll
			m.applyFiltersAndSort()
			if cur := m.currentRepo(); cur != nil {
				cmds = append(cmds, m.loadTokeiCmd(cur.Path))
			}

		case "u":
			if m.filterMode == models.FilterUncommitted {
				m.filterMode = models.FilterAll
			} else {
				m.filterMode = models.FilterUncommitted
			}
			m.applyFiltersAndSort()
			if cur := m.currentRepo(); cur != nil {
				cmds = append(cmds, m.loadTokeiCmd(cur.Path))
			}

		case "g":
			if m.filterMode == models.FilterGitHub {
				m.filterMode = models.FilterAll
			} else {
				m.filterMode = models.FilterGitHub
			}
			m.applyFiltersAndSort()
			if cur := m.currentRepo(); cur != nil {
				cmds = append(cmds, m.loadTokeiCmd(cur.Path))
			}

		case "l":
			if m.filterMode == models.FilterLocal {
				m.filterMode = models.FilterAll
			} else {
				m.filterMode = models.FilterLocal
			}
			m.applyFiltersAndSort()
			if cur := m.currentRepo(); cur != nil {
				cmds = append(cmds, m.loadTokeiCmd(cur.Path))
			}

		case "p":
			if m.filterMode == models.FilterPublic {
				m.filterMode = models.FilterAll
			} else {
				m.filterMode = models.FilterPublic
			}
			m.applyFiltersAndSort()
			if cur := m.currentRepo(); cur != nil {
				cmds = append(cmds, m.loadTokeiCmd(cur.Path))
			}

		case "P":
			if m.filterMode == models.FilterPrivate {
				m.filterMode = models.FilterAll
			} else {
				m.filterMode = models.FilterPrivate
			}
			m.applyFiltersAndSort()
			if cur := m.currentRepo(); cur != nil {
				cmds = append(cmds, m.loadTokeiCmd(cur.Path))
			}

		// Sorting
		case "s":
			m.sortMode = (m.sortMode + 1) % 5
			m.applyFiltersAndSort()
		// Open in browser
		case "o":
			if cur := m.currentRepo(); cur != nil && cur.GitHubWebURL != "" {
				_ = openURL(cur.GitHubWebURL)
				m.statusMsg = fmt.Sprintf("Opened %s in browser", cur.GitHubSlug)
				m.statusIsError = false
				cmds = append(cmds, m.clearStatusAfterDelay())
			}

		// Deletion modal
		case "d", "x":
			if cur := m.currentRepo(); cur != nil {
				m.openDeleteModal()
			}

		// Refresh
		case "r":
			m.statusMsg = "Rescanning repositories..."
			m.statusIsError = false
			return m, func() tea.Msg {
				repos, err := git.ScanRepositories(m.rootDir, 6)
				if err != nil {
					return nil
				}
				return ghEnrichMsg{repos: repos}
			}
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) clearStatusAfterDelay() tea.Cmd {
	return tea.Tick(4*time.Second, func(t time.Time) tea.Msg {
		return statusClearMsg{}
	})
}

func (m *Model) handleMouse(msg tea.MouseMsg) tea.Cmd {
	switch msg.Action {
	case tea.MouseActionPress:
		if msg.Button == tea.MouseButtonLeft {
			// Calculate table header height
			headerH := 5
			if m.searching || m.searchInput.Value() != "" {
				headerH = 6
			}
			splitW := m.leftPaneWidth()

			// If clicked in left pane (table rows)
			if msg.X < splitW && msg.Y >= headerH && msg.Y < headerH+m.tableBodyHeight() {
				clickedRow := (msg.Y - headerH) + m.scrollOffset
				if clickedRow >= 0 && clickedRow < len(m.items) {
					m.noSelection = false
					item := m.items[clickedRow]
					if item.Kind == ItemKindFolder {
						m.cursor = clickedRow
						m.collapsedFolders[item.Folder] = !m.collapsedFolders[item.Folder]
						m.applyFiltersAndSort()
						return m.loadFolderTokeiCmd(item.Folder)
					}
					if item.HasChildren && msg.X < 8 {
						folder := strings.Split(filepath.ToSlash(item.Repo.RelPath), "/")[0]
						m.cursor = clickedRow
						m.collapsedFolders[folder] = !m.collapsedFolders[folder]
						m.applyFiltersAndSort()
						return nil
					}
					if m.cursor == clickedRow {
						m.showInspectorFull = !m.showInspectorFull
					} else {
						m.cursor = clickedRow
					}
					if cur := m.currentRepo(); cur != nil {
						return m.loadTokeiCmd(cur.Path)
					}
				} else if clickedRow >= len(m.items) {
					m.noSelection = true
					m.cursor = -1
					return m.loadAllTokeiCmd()
				}
			} else if msg.X < splitW && msg.Y < headerH {
				m.noSelection = true
				m.cursor = -1
				return m.loadAllTokeiCmd()
			}
		}
	case tea.MouseActionMotion:
		// Optional hover handling
	}

	// Scroll wheel
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		if m.showInspectorFull && m.inspectorScroll > 0 {
			m.inspectorScroll--
		} else if m.cursor > 0 {
			m.selectRow(m.cursor - 1)
			if cur := m.currentRepo(); cur != nil {
				return m.loadTokeiCmd(cur.Path)
			} else if item := m.currentItem(); item != nil && item.Kind == ItemKindFolder {
				return m.loadFolderTokeiCmd(item.Folder)
			}
		} else if m.cursor == 0 && !m.noSelection {
			return m.deselect()
		}
	case tea.MouseButtonWheelDown:
		if m.showInspectorFull {
			m.inspectorScroll++
		} else if m.noSelection && len(m.items) > 0 {
			m.selectRow(0)
			if cur := m.currentRepo(); cur != nil {
				return m.loadTokeiCmd(cur.Path)
			} else if item := m.currentItem(); item != nil && item.Kind == ItemKindFolder {
				return m.loadFolderTokeiCmd(item.Folder)
			}
		} else if m.cursor < len(m.items)-1 {
			m.selectRow(m.cursor + 1)
			if cur := m.currentRepo(); cur != nil {
				return m.loadTokeiCmd(cur.Path)
			} else if item := m.currentItem(); item != nil && item.Kind == ItemKindFolder {
				return m.loadFolderTokeiCmd(item.Folder)
			}
		}
	}

	return nil
}

func (m *Model) openDeleteModal() {
	cur := m.currentRepo()
	if cur == nil {
		return
	}
	m.showDeleteModal = true
	m.deleteStep = 0
	m.deleteChoice = 0
	m.deleteConfirmInput.SetValue("")
	m.deleteConfirmInput.Blur()
}

func (m Model) handleDeleteKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	cur := m.currentRepo()
	if cur == nil {
		m.showDeleteModal = false
		return m, nil
	}

	// Step 0: Choose option (1: Local, 2: GitHub, 3: Both)
	if m.deleteStep == 0 {
		switch msg.String() {
		case "esc", "c", "q":
			m.showDeleteModal = false
			return m, nil
		case "tab":
			m.deleteChoice = (m.deleteChoice % 3) + 1
			return m, nil
		case "shift+tab", "backtab":
			m.deleteChoice = ((m.deleteChoice + 1) % 3) + 1
			return m, nil
		case "enter":
			if m.deleteChoice > 0 {
				if (m.deleteChoice == 2 || m.deleteChoice == 3) && (!cur.IsGitHub || cur.GitHubSlug == "") {
					m.statusMsg = "Cannot delete on GitHub: repository has no GitHub remote"
					m.statusIsError = true
					return m, nil
				}
				m.deleteStep = 1
				m.deleteConfirmInput.Focus()
				return m, textinput.Blink
			}
		case "1":
			m.deleteChoice = 1 // Local only
			m.deleteStep = 1
			m.deleteConfirmInput.Focus()
			return m, textinput.Blink
		case "2":
			if !cur.IsGitHub || cur.GitHubSlug == "" {
				m.statusMsg = "Cannot delete on GitHub: repository has no GitHub remote"
				m.statusIsError = true
				return m, nil
			}
			m.deleteChoice = 2 // GitHub only
			m.deleteStep = 1
			m.deleteConfirmInput.Focus()
			return m, textinput.Blink
		case "3":
			if !cur.IsGitHub || cur.GitHubSlug == "" {
				m.statusMsg = "Cannot delete on GitHub: repository has no GitHub remote"
				m.statusIsError = true
				return m, nil
			}
			m.deleteChoice = 3 // Both
			m.deleteStep = 1
			m.deleteConfirmInput.Focus()
			return m, textinput.Blink
		}
		return m, nil
	}

	// Step 1: Confirm by typing repo name or pressing 'y'
	switch msg.String() {
	case "esc":
		m.showDeleteModal = false
		return m, nil
	case "enter":
		val := strings.TrimSpace(m.deleteConfirmInput.Value())
		if val != cur.Name && val != "y" && val != "yes" {
			m.statusMsg = fmt.Sprintf("Confirmation mismatched. Expected %q or 'y'. Deletion aborted.", cur.Name)
			m.statusIsError = true
			m.showDeleteModal = false
			return m, nil
		}

		// Perform deletion
		deleteLocal := m.deleteChoice == 1 || m.deleteChoice == 3
		deleteGitHub := m.deleteChoice == 2 || m.deleteChoice == 3

		localErr, githubErr := git.DeleteRepo(cur, deleteLocal, deleteGitHub)
		m.showDeleteModal = false

		if githubErr != nil && localErr != nil {
			m.statusMsg = fmt.Sprintf("Local failed: %v | GitHub failed: %v", localErr, githubErr)
			m.statusIsError = true
		} else if githubErr != nil {
			m.statusMsg = fmt.Sprintf("GitHub deletion failed: %v", githubErr)
			m.statusIsError = true
		} else if localErr != nil {
			m.statusMsg = fmt.Sprintf("Local deletion failed: %v", localErr)
			m.statusIsError = true
		} else {
			// Success
			scope := "locally"
			if deleteLocal && deleteGitHub {
				scope = "locally and on GitHub"
			} else if deleteGitHub {
				scope = "on GitHub"
			}
			m.statusMsg = fmt.Sprintf("Successfully deleted %s %s", cur.Name, scope)
			m.statusIsError = false

			if deleteLocal {
				// Remove from allRepos
				var remaining []*models.Repo
				for _, r := range m.allRepos {
					if r.Path != cur.Path {
						remaining = append(remaining, r)
					}
				}
				m.allRepos = remaining
				delete(m.tokeiCache, cur.Path)
				m.applyFiltersAndSort()
			}
		}

		return m, m.clearStatusAfterDelay()

	default:
		var cmd tea.Cmd
		m.deleteConfirmInput, cmd = m.deleteConfirmInput.Update(msg)
		return m, cmd
	}
}

func (m Model) leftPaneWidth() int {
	if m.width < 105 {
		return m.width - 2
	}
	w := int(float64(m.width) * 0.65)
	if w < 76 {
		w = 76
	}
	if w > m.width-36 {
		w = m.width - 36
	}
	return w
}

func (m Model) rightPaneWidth() int {
	if m.width < 105 {
		return m.width - 4
	}
	w := m.width - m.leftPaneWidth() - 7
	if w < 20 {
		w = 20
	}
	return w
}

func openURL(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
		args = []string{url}
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start", url}
	default:
		cmd = "xdg-open"
		args = []string{url}
	}
	return exec.Command(cmd, args...).Start()
}

// View renders the TUI.
func (m Model) View() string {
	var b strings.Builder

	// 1. Header & Summary Stats
	b.WriteString(m.renderHeader())
	b.WriteString("\n")

	// 2. Filter & Sort bar
	b.WriteString(m.renderControls())
	b.WriteString("\n")

	// 3. Search Bar (if active or has value)
	if m.searching || m.searchInput.Value() != "" {
		b.WriteString(m.searchInput.View())
		b.WriteString("\n")
	}

	// 4. Modal or Main View
	if m.showDeleteModal {
		b.WriteString(m.renderDeleteModal())
		b.WriteString("\n")
	} else if m.showInspectorFull || m.width < 105 && m.showInspectorFull {
		b.WriteString(m.renderFullInspector())
		b.WriteString("\n")
	} else {
		b.WriteString(m.renderSplitView())
		b.WriteString("\n")
	}

	// 5. Status / Help Bar
	b.WriteString(m.renderFooter())

	return b.String()
}

func (m Model) renderHeader() string {
	var cleanCnt, dirtyCnt, ghCnt, pubCnt, privCnt, localCnt int
	for _, r := range m.allRepos {
		if r.HasUncommitted {
			dirtyCnt++
		} else {
			cleanCnt++
		}
		if r.IsGitHub {
			ghCnt++
			if r.GitHubVisibility == models.VisibilityPublic {
				pubCnt++
			} else if r.GitHubVisibility == models.VisibilityPrivate {
				privCnt++
			}
		} else {
			localCnt++
		}
	}

	displayPath := m.rootDir
	if strings.HasPrefix(displayPath, "/Users/") {
		parts := strings.SplitN(displayPath, "/", 4)
		if len(parts) >= 3 {
			displayPath = "~/" + parts[len(parts)-1]
		}
	}

	title := titleStyle.Render("REPO-LS")
	pathStr := headerPathStyle.Render("📁 " + displayPath)

	stats := fmt.Sprintf("%d repos · %s · %s · %s (%s, %s) · %s",
		len(m.allRepos),
		badgeClean+" "+fmt.Sprint(cleanCnt),
		badgeDirty+" "+fmt.Sprint(dirtyCnt),
		accentStyle.Render(fmt.Sprintf("%d GitHub", ghCnt)),
		successStyle.Render(fmt.Sprintf("%d pub", pubCnt)),
		warningStyle.Render(fmt.Sprintf("%d priv", privCnt)),
		dimStyle.Render(fmt.Sprintf("%d local", localCnt)),
	)

	line := fmt.Sprintf("%s  %s  %s", title, pathStr, stats)
	if lipgloss.Width(line) > m.width {
		pathStr = headerPathStyle.Render("📁 " + filepath.Base(displayPath))
		line = fmt.Sprintf("%s  %s  %s", title, pathStr, stats)
	}
	if lipgloss.Width(line) > m.width {
		line = fmt.Sprintf("%s  %s", title, stats)
	}
	return line
}

func (m Model) renderControls() string {
	filterItems := []string{
		m.formatFilterTab("All", models.FilterAll, "a"),
		m.formatFilterTab("Dirty", models.FilterUncommitted, "u"),
		m.formatFilterTab("Clean", models.FilterClean, ""),
		m.formatFilterTab("GitHub", models.FilterGitHub, "g"),
		m.formatFilterTab("Local", models.FilterLocal, "l"),
		m.formatFilterTab("Public", models.FilterPublic, "p"),
		m.formatFilterTab("Private", models.FilterPrivate, "P"),
	}

	filterBar := strings.Join(filterItems, " ")
	sortStr := fmt.Sprintf("Sort: %s (s)", accentStyle.Render(m.sortMode.String()))
	tabHint := dimStyle.Render("Tab: filter")
	searchHint := dimStyle.Render("Search: (/)")

	countStr := fmt.Sprintf("%d/%d", len(m.filtered), len(m.allRepos))
	if len(m.filtered) != len(m.allRepos) {
		countStr = warningStyle.Render(countStr)
	} else {
		countStr = dimStyle.Render(countStr)
	}

	line := fmt.Sprintf("%s   %s   %s   %s   %s", filterBar, sortStr, tabHint, searchHint, countStr)
	if lipgloss.Width(line) > m.width {
		line = fmt.Sprintf("%s   %s   %s   %s", filterBar, sortStr, tabHint, countStr)
	}
	if lipgloss.Width(line) > m.width {
		line = fmt.Sprintf("%s   %s   %s", filterBar, sortStr, countStr)
	}
	if lipgloss.Width(line) > m.width {
		line = fmt.Sprintf("%s   %s", filterBar, sortStr)
	}
	return line
}

func (m Model) formatFilterTab(label string, mode models.FilterMode, key string) string {
	text := label
	if key != "" {
		text = fmt.Sprintf("[%s] %s", key, label)
	}
	if m.filterMode == mode {
		return lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(purpleColor).
			Padding(0, 1).
			Render(text)
	}
	return dimStyle.Render(text)
}

func (m Model) renderSplitView() string {
	leftW := m.leftPaneWidth()
	rightW := m.rightPaneWidth()
	bodyH := m.tableBodyHeight()

	leftContent := m.renderTable(leftW, bodyH)
	rightContent := m.renderInspector(rightW, bodyH)

	leftPane := paneLeftStyle.Width(leftW).Height(bodyH).Render(leftContent)
	rightPane := paneRightStyle.Width(rightW).Height(bodyH).Render(rightContent)

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPane, " ", rightPane)
}

func (m Model) renderTable(width int, height int) string {
	var lines []string

	const (
		stColW       = 2  // "ST"
		branchColW   = 12 // "BRANCH"
		visColW      = 10 // "VISIBILITY"
		createdColW  = 12 // "DATE-CREATED"
		modifiedColW = 13 // "LAST-MODIFIED"
		fixedCols    = 1 + stColW + 2 + 2 + branchColW + 2 + visColW + 2 + createdColW + 2 + modifiedColW // = 60
	)

	nameColW := width - fixedCols
	if nameColW < 16 {
		nameColW = 16
	}

	// Table Header
	header := fmt.Sprintf(" %-2s  %s  %s  %s  %s  %s",
		"ST",
		padRight("PROJECT", nameColW),
		padRight("BRANCH", branchColW),
		padRight("VISIBILITY", visColW),
		padRight("DATE-CREATED", createdColW),
		padRight("LAST-MODIFIED", modifiedColW),
	)
	if lipgloss.Width(header) > width {
		header = ansi.Truncate(header, width, "")
	}
	lines = append(lines, tableHeaderStyle.Width(width).Render(header))

	if len(m.items) == 0 {
		lines = append(lines, dimStyle.Render("\n   No matching repositories found."))
		for len(lines) < height {
			lines = append(lines, "")
		}
		return strings.Join(lines, "\n")
	}

	// Rows
	end := m.scrollOffset + height - 1
	if end > len(m.items) {
		end = len(m.items)
	}

	for i := m.scrollOffset; i < end; i++ {
		item := m.items[i]
		isSelected := i == m.cursor

		if item.Kind == ItemKindFolder {
			icon := "▼"
			if !item.IsExpanded {
				icon = "▶"
			}
			dirtyHint := ""
			if item.DirtyCount > 0 {
				dirtyHint = "  " + dangerStyle.Render("●") + fmt.Sprintf(" %d dirty", item.DirtyCount)
			}
			folderText := fmt.Sprintf(" %s   📁 %s (%d repos)%s", icon, item.Folder, item.RepoCount, dirtyHint)
			if lipgloss.Width(folderText) > width {
				folderText = ansi.Truncate(folderText, width, "")
			}
			if isSelected {
				lines = append(lines, selectedRowStyle.Width(width).Render(fmt.Sprintf(" %s   📁 %s (%d repos)%s", icon, item.Folder, item.RepoCount, dirtyHint)))
			} else {
				lines = append(lines, accentStyle.Render(fmt.Sprintf(" %s   📁 %s (%d repos)", icon, item.Folder, item.RepoCount))+dirtyHint)
			}
			continue
		}

		r := item.Repo
		if r == nil {
			continue
		}

		// Visibility
		visStr := "local"
		if r.IsGitHub {
			if r.GitHubVisibility == models.VisibilityPublic {
				visStr = "public"
			} else if r.GitHubVisibility == models.VisibilityPrivate {
				visStr = "private"
			} else {
				visStr = "github"
			}
		}

		// Relative commit / dates
		createdRel := models.RelativeTime(r.DateCreated)
		modifiedRel := models.RelativeTime(r.LastModified)

		// Name formatting
		dispName := r.RelPath
		if item.HasChildren {
			icon := "▼"
			if !item.IsExpanded {
				icon = "▶"
			}
			suffix := fmt.Sprintf(" (+%d repos)", item.RepoCount)
			avail := nameColW - lipgloss.Width(suffix) - 2
			trimmedName := r.Name
			if avail > 0 && lipgloss.Width(trimmedName) > avail {
				trimmedName = ansi.Truncate(trimmedName, avail, "…")
			}
			dispName = fmt.Sprintf("%s %s%s", icon, trimmedName, suffix)
		} else if item.IsChild {
			dispName = "  " + r.Name
			if lipgloss.Width(dispName) > nameColW {
				dispName = ansi.Truncate(dispName, nameColW, "…")
			}
		} else {
			if lipgloss.Width(dispName) > nameColW {
				dispName = ansi.Truncate(dispName, nameColW, "…")
			}
		}

		branch := r.Branch
		if lipgloss.Width(branch) > branchColW {
			branch = ansi.Truncate(branch, branchColW, "…")
		}

		rowContent := fmt.Sprintf("%s  %s  %s  %s  %s",
			padRight(dispName, nameColW),
			padRight(branch, branchColW),
			padRight(visStr, visColW),
			padRight(createdRel, createdColW),
			padRight(modifiedRel, modifiedColW),
		)

		if isSelected {
			stDot := dangerStyle.Background(lipgloss.Color("#2A2A3D")).Render("●")
			if !r.HasUncommitted {
				stDot = successStyle.Background(lipgloss.Color("#2A2A3D")).Render("✓")
			}
			lines = append(lines, selectedRowStyle.Render(" ")+stDot+selectedRowStyle.Render("   ")+selectedRowStyle.Width(width-5).Render(rowContent))
		} else {
			stDot := dangerStyle.Render("●")
			if !r.HasUncommitted {
				stDot = successStyle.Render("✓")
			}
			lines = append(lines, " "+stDot+"   "+rowContent)
		}
	}

	for len(lines) < height {
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}

func (m Model) renderInspector(width int, height int) string {
	if m.cursor == -1 || len(m.items) == 0 {
		return m.renderAllReposSummary(width, height)
	}

	item := m.currentItem()
	if item == nil {
		return m.renderAllReposSummary(width, height)
	}

	if item.Kind == ItemKindFolder {
		return m.renderFolderSummary(item.Folder, width, height)
	}

	cur := item.Repo
	if cur == nil {
		return m.renderAllReposSummary(width, height)
	}

	return m.renderRepoInspector(cur, width, height)
}

func (m Model) renderAllReposSummary(width int, height int) string {
	var lines []string

	title := accentStyle.Render("ALL REPOSITORIES SUMMARY")
	lines = append(lines, title)

	dispRoot := m.rootDir
	if strings.HasPrefix(dispRoot, "/Users/") {
		parts := strings.SplitN(dispRoot, "/", 4)
		if len(parts) >= 3 {
			dispRoot = "~/" + parts[len(parts)-1]
		}
	}
	lines = append(lines, dimStyle.Render(fmt.Sprintf("📁 %s · %d total repositories", dispRoot, len(m.allRepos))))
	lines = append(lines, subtleStyle.Render(strings.Repeat("─", width-2)))

	var cleanCnt, dirtyCnt, ghCnt, pubCnt, privCnt, localCnt, totalCommits, totalUncommittedFiles int
	var mostRecentRepo *models.Repo
	for _, r := range m.allRepos {
		if r.HasUncommitted {
			dirtyCnt++
			totalUncommittedFiles += r.UncommittedCount
		} else {
			cleanCnt++
		}
		if r.IsGitHub {
			ghCnt++
			if r.GitHubVisibility == models.VisibilityPublic {
				pubCnt++
			} else if r.GitHubVisibility == models.VisibilityPrivate {
				privCnt++
			}
		} else {
			localCnt++
		}
		totalCommits += r.CommitCount
		if mostRecentRepo == nil || (r.LastModified.After(mostRecentRepo.LastModified)) {
			mostRecentRepo = r
		}
	}

	statusLine := fmt.Sprintf("%s %d clean  ·  %s %d dirty", badgeClean, cleanCnt, badgeDirty, dirtyCnt)
	if dirtyCnt > 0 {
		statusLine += fmt.Sprintf(" (%d uncommitted files)", totalUncommittedFiles)
	}
	lines = append(lines, statusLine)

	visLine := fmt.Sprintf("Visibility: %s (%s, %s) · %s",
		accentStyle.Render(fmt.Sprintf("%d GitHub", ghCnt)),
		successStyle.Render(fmt.Sprintf("%d public", pubCnt)),
		warningStyle.Render(fmt.Sprintf("%d private", privCnt)),
		dimStyle.Render(fmt.Sprintf("%d local", localCnt)),
	)
	lines = append(lines, visLine)
	lines = append(lines, dimStyle.Render(fmt.Sprintf("Total Commits: %s across all branches", formatNumber(totalCommits))))

	if mostRecentRepo != nil && !mostRecentRepo.LastModified.IsZero() {
		recentDesc := fmt.Sprintf("Latest Activity: %s (%s)", accentStyle.Render(mostRecentRepo.Name), models.RelativeTime(mostRecentRepo.LastModified))
		if mostRecentRepo.LastCommit != nil {
			recentDesc += fmt.Sprintf(" - %s", dimStyle.Render(mostRecentRepo.LastCommit.Relative))
		}
		lines = append(lines, recentDesc)
	}

	lines = append(lines, subtleStyle.Render(strings.Repeat("─", width-2)))
	lines = append(lines, accentStyle.Render("Code Stats (tokei - all repos):"))

	stats, hasCached := m.tokeiCache[allReposTokeiKey]
	if !hasCached && m.loadingTokei[allReposTokeiKey] {
		lines = append(lines, dimStyle.Render("  ⏳ Analyzing all repositories with tokei..."))
	} else if stats == nil || stats.Err != "" {
		errMsg := "No stats available"
		if stats != nil && stats.Err != "" {
			errMsg = stats.Err
		}
		lines = append(lines, dimStyle.Render("  "+errMsg))
	} else {
		tot := stats.Total
		summaryLine := fmt.Sprintf("  Total: %s lines (%s code, %s comments, %s blanks, %d files)",
			formatNumber(tot.Lines),
			successStyle.Render(formatNumber(tot.Code)),
			dimStyle.Render(formatNumber(tot.Comments)),
			dimStyle.Render(formatNumber(tot.Blanks)),
			tot.Files,
		)
		lines = append(lines, summaryLine)

		maxLangs := 8
		if len(stats.Languages) < maxLangs {
			maxLangs = len(stats.Languages)
		}
		if maxLangs > 0 {
			lines = append(lines, "")
			langColors := []lipgloss.Color{
				cyanColor, purpleColor, greenColor, orangeColor, yellowColor,
				lipgloss.Color("#FF5FD7"), lipgloss.Color("#5FAFFF"), lipgloss.Color("#AFD700"),
			}
			barWidth := width - 38
			if barWidth < 10 {
				barWidth = 10
			}
			if barWidth > 20 {
				barWidth = 20
			}
			for i := 0; i < maxLangs; i++ {
				l := stats.Languages[i]
				pct := 0.0
				if tot.Code > 0 {
					pct = float64(l.Code) / float64(tot.Code) * 100
				}
				bar := ProgressBar(barWidth, pct/100.0, langColors[i%len(langColors)])
				langLine := fmt.Sprintf("  %-12s %s  %5.1f%% (%s lines, %d files)",
					l.Name,
					bar,
					pct,
					formatNumber(l.Code),
					l.Files,
				)
				if len(langLine) > width {
					langLine = langLine[:width]
				}
				lines = append(lines, langLine)
			}
		}
	}

	lines = append(lines, "")
	lines = append(lines, dimStyle.Render("💡 Tip: Press ↓/↑ or click a repo to inspect; Tab cycles filters"))
	lines = append(lines, dimStyle.Render("💡 Tip: Press Space to collapse/expand folders (e.g. ARCHIVE)"))

	return strings.Join(lines, "\n")
}

func (m Model) renderFolderSummary(folder string, width int, height int) string {
	var lines []string

	title := accentStyle.Render(fmt.Sprintf("FOLDER: %s", folder))
	lines = append(lines, title)

	var reposInFolder []*models.Repo
	for _, r := range m.allRepos {
		if strings.HasPrefix(filepath.ToSlash(r.RelPath), folder+"/") {
			reposInFolder = append(reposInFolder, r)
		}
	}

	lines = append(lines, dimStyle.Render(fmt.Sprintf("Contains %d repositories", len(reposInFolder))))
	lines = append(lines, subtleStyle.Render(strings.Repeat("─", width-2)))

	var cleanCnt, dirtyCnt, ghCnt, pubCnt, privCnt, localCnt int
	for _, r := range reposInFolder {
		if r.HasUncommitted {
			dirtyCnt++
		} else {
			cleanCnt++
		}
		if r.IsGitHub {
			ghCnt++
			if r.GitHubVisibility == models.VisibilityPublic {
				pubCnt++
			} else if r.GitHubVisibility == models.VisibilityPrivate {
				privCnt++
			}
		} else {
			localCnt++
		}
	}

	lines = append(lines, fmt.Sprintf("Status: %s %d clean  ·  %s %d dirty", badgeClean, cleanCnt, badgeDirty, dirtyCnt))
	lines = append(lines, fmt.Sprintf("Visibility: %d GitHub (%d public, %d private)  ·  %d local", ghCnt, pubCnt, privCnt, localCnt))

	lines = append(lines, subtleStyle.Render(strings.Repeat("─", width-2)))
	lines = append(lines, accentStyle.Render(fmt.Sprintf("Code Stats (tokei - %s):", folder)))

	key := "__folder__" + folder
	stats, hasCached := m.tokeiCache[key]
	if !hasCached && m.loadingTokei[key] {
		lines = append(lines, dimStyle.Render("  ⏳ Analyzing folder with tokei..."))
	} else if stats == nil || stats.Err != "" {
		errMsg := "No stats available"
		if stats != nil && stats.Err != "" {
			errMsg = stats.Err
		}
		lines = append(lines, dimStyle.Render("  "+errMsg))
	} else {
		tot := stats.Total
		summaryLine := fmt.Sprintf("  Total: %s lines (%s code, %s comments, %d files)",
			formatNumber(tot.Lines),
			successStyle.Render(formatNumber(tot.Code)),
			dimStyle.Render(formatNumber(tot.Comments)),
			tot.Files,
		)
		lines = append(lines, summaryLine)

		maxLangs := 6
		if len(stats.Languages) < maxLangs {
			maxLangs = len(stats.Languages)
		}
		if maxLangs > 0 {
			lines = append(lines, "")
			langColors := []lipgloss.Color{cyanColor, purpleColor, greenColor, orangeColor, yellowColor}
			barWidth := width - 36
			if barWidth < 10 {
				barWidth = 10
			}
			if barWidth > 20 {
				barWidth = 20
			}
			for i := 0; i < maxLangs; i++ {
				l := stats.Languages[i]
				pct := 0.0
				if tot.Code > 0 {
					pct = float64(l.Code) / float64(tot.Code) * 100
				}
				bar := ProgressBar(barWidth, pct/100.0, langColors[i%len(langColors)])
				langLine := fmt.Sprintf("  %-12s %6s code %5.1f%% %s",
					l.Name,
					formatNumber(l.Code),
					pct,
					bar,
				)
				if len(langLine) > width {
					langLine = langLine[:width]
				}
				lines = append(lines, langLine)
			}
		}
	}

	lines = append(lines, "")
	lines = append(lines, dimStyle.Render("💡 Tip: Press Space or Enter to collapse/expand this folder"))

	return strings.Join(lines, "\n")
}

func (m Model) renderRepoInspector(cur *models.Repo, width int, height int) string {
	var lines []string

	// Title / Name
	title := accentStyle.Render(cur.Name)
	if cur.IsGitHub {
		title += " " + RenderVisibilityBadge(cur.GitHubVisibility)
	} else {
		title += " " + badgeLocal
	}
	lines = append(lines, title)

	// Path
	pathStr := dimStyle.Render(cur.Path)
	if len(pathStr) > width {
		pathStr = pathStr[:width]
	}
	lines = append(lines, pathStr)

	// Dates
	var dateParts []string
	if !cur.DateCreated.IsZero() {
		dateParts = append(dateParts, fmt.Sprintf("Created: %s (%s)", cur.DateCreated.Format("2006-01-02"), models.RelativeTime(cur.DateCreated)))
	}
	if !cur.LastModified.IsZero() {
		dateParts = append(dateParts, fmt.Sprintf("Modified: %s (%s)", cur.LastModified.Format("2006-01-02 15:04"), models.RelativeTime(cur.LastModified)))
	}
	if len(dateParts) > 0 {
		lines = append(lines, dimStyle.Render(strings.Join(dateParts, "  ·  ")))
	}

	lines = append(lines, subtleStyle.Render(strings.Repeat("─", width-2)))

	// Git Status
	statusLine := RenderStatusBadge(cur.HasUncommitted, cur.UncommittedCount)
	if cur.HasUncommitted {
		statusLine += fmt.Sprintf(" (%d uncommitted changes)", cur.UncommittedCount)
	} else {
		statusLine += " (working tree clean)"
	}
	lines = append(lines, fmt.Sprintf("Branch: %s  ·  %s", accentStyle.Render(cur.Branch), statusLine))

	// Last Commit
	if cur.LastCommit != nil {
		lines = append(lines, fmt.Sprintf("Latest: %s (%s by %s)",
			warningStyle.Render(cur.LastCommit.Hash),
			cur.LastCommit.Relative,
			cur.LastCommit.Author,
		))
		msg := cur.LastCommit.Message
		if len(msg) > width-4 {
			msg = msg[:width-7] + "..."
		}
		lines = append(lines, dimStyle.Render("  \""+msg+"\""))
	} else {
		lines = append(lines, dimStyle.Render("Latest: No commits"))
	}

	// Remote
	if cur.IsGitHub {
		lines = append(lines, fmt.Sprintf("GitHub: %s %s",
			accentStyle.Render(cur.GitHubSlug),
			dimStyle.Render("[o: open in browser]"),
		))
	} else if cur.RemoteURL != "" {
		rURL := cur.RemoteURL
		if len(rURL) > width-10 {
			rURL = rURL[:width-13] + "..."
		}
		lines = append(lines, fmt.Sprintf("Remote: %s (%s)", dimStyle.Render(cur.RemoteName), rURL))
	} else {
		lines = append(lines, dimStyle.Render("Remote: Local only (no remote)"))
	}

	// Uncommitted files preview (if dirty)
	if cur.HasUncommitted && len(cur.UncommittedDetails) > 0 {
		lines = append(lines, subtleStyle.Render(strings.Repeat("─", width-2)))
		lines = append(lines, warningStyle.Render(fmt.Sprintf("Changed Files (%d):", cur.UncommittedCount)))
		maxFiles := 5
		for i, f := range cur.UncommittedDetails {
			if i >= maxFiles {
				lines = append(lines, dimStyle.Render(fmt.Sprintf("  ... and %d more files", len(cur.UncommittedDetails)-maxFiles)))
				break
			}
			fStr := "  " + f
			if len(fStr) > width {
				fStr = fStr[:width]
			}
			lines = append(lines, dangerStyle.Render(fStr))
		}
	}

	// Tokei Code Statistics
	lines = append(lines, subtleStyle.Render(strings.Repeat("─", width-2)))
	lines = append(lines, accentStyle.Render("Code Stats (tokei):"))

	stats, hasCached := m.tokeiCache[cur.Path]
	if !hasCached && m.loadingTokei[cur.Path] {
		lines = append(lines, dimStyle.Render("  ⏳ Analyzing codebase with tokei..."))
	} else if stats == nil || stats.Err != "" {
		errMsg := "No stats available"
		if stats != nil && stats.Err != "" {
			errMsg = stats.Err
		}
		lines = append(lines, dimStyle.Render("  "+errMsg))
	} else {
		tot := stats.Total
		summaryLine := fmt.Sprintf("  Total: %s lines (%s code, %s comments, %d files)",
			formatNumber(tot.Lines),
			successStyle.Render(formatNumber(tot.Code)),
			dimStyle.Render(formatNumber(tot.Comments)),
			tot.Files,
		)
		lines = append(lines, summaryLine)

		// Top languages
		topCount := 6
		barWidth := width - 36
		if barWidth < 10 {
			barWidth = 10
		}
		if barWidth > 20 {
			barWidth = 20
		}

		for i, l := range stats.Languages {
			if i >= topCount {
				break
			}
			pct := 0.0
			if tot.Code > 0 {
				pct = (float64(l.Code) / float64(tot.Code)) * 100
			}
			bar := ProgressBar(barWidth, pct/100.0, cyanColor)

			line := fmt.Sprintf("  %-12s %6s code %5.1f%% %s",
				l.Name,
				formatNumber(l.Code),
				pct,
				bar,
			)
			if len(line) > width {
				line = line[:width]
			}
			lines = append(lines, line)
		}
	}

	return strings.Join(lines, "\n")
}

func (m Model) renderFullInspector() string {
	w := m.width - 4
	var lines []string

	title := "REPOSITORY INSPECTION"
	if cur := m.currentRepo(); cur != nil {
		title += ": " + cur.Name
	} else if item := m.currentItem(); item != nil && item.Kind == ItemKindFolder {
		title = "FOLDER INSPECTION: " + item.Folder
	} else {
		title = "ALL REPOSITORIES SUMMARY"
	}

	lines = append(lines, headerStyle.Render(title))
	lines = append(lines, "")
	lines = append(lines, m.renderInspector(w, m.height-6))
	lines = append(lines, "")
	lines = append(lines, dimStyle.Render("[Esc/Enter] Return to table  [o] Open GitHub  [d] Delete repo"))

	return strings.Join(lines, "\n")
}

func (m Model) renderDeleteModal() string {
	cur := m.currentRepo()
	if cur == nil {
		return ""
	}

	var content string
	if m.deleteStep == 0 {
		ghOption := "[2] Delete on GitHub only (gh repo delete)"
		bothOption := "[3] Delete BOTH locally AND on GitHub"
		if !cur.IsGitHub {
			ghOption = dimStyle.Render("[2] Delete on GitHub only (Disabled: not a GitHub repo)")
			bothOption = dimStyle.Render("[3] Delete BOTH locally AND on GitHub (Disabled)")
		}

		opt1 := "[1] Delete Locally only (rm -rf)"
		if m.deleteChoice == 1 {
			opt1 = accentStyle.Render("▶ [1] Delete Locally only (rm -rf)")
		} else if m.deleteChoice == 2 {
			ghOption = accentStyle.Render("▶ " + ghOption)
		} else if m.deleteChoice == 3 {
			bothOption = accentStyle.Render("▶ " + bothOption)
		}

		content = fmt.Sprintf(
			"🗑️  DELETE REPOSITORY: %s\n\n"+
				"Path:   %s\n"+
				"Remote: %s\n\n"+
				"Choose deletion action:\n"+
				"  %s\n"+
				"  %s\n"+
				"  %s\n\n"+
				"[Tab] Cycle Options    [Enter] Select    [1/2/3] Choose    [Esc] Cancel",
			dangerStyle.Render(cur.Name),
			dimStyle.Render(cur.Path),
			accentStyle.Render(cur.RemoteURL),
			opt1,
			ghOption,
			bothOption,
		)
	} else {
		actionDesc := "locally"
		if m.deleteChoice == 2 {
			actionDesc = "on GitHub"
		} else if m.deleteChoice == 3 {
			actionDesc = "LOCALLY and on GITHUB"
		}

		content = fmt.Sprintf(
			"⚠️  CONFIRM PERMANENT DELETION\n\n"+
				"You are about to delete %s %s!\n\n"+
				"To proceed, type the repository name %s or 'y':\n\n"+
				"%s\n\n"+
				"[Enter] Confirm Deletion    [Esc] Cancel",
			dangerStyle.Render(cur.Name),
			dangerStyle.Render(actionDesc),
			accentStyle.Render(cur.Name),
			m.deleteConfirmInput.View(),
		)
	}

	return modalStyle.Width(m.width - 10).Render(content)
}

func (m Model) renderFooter() string {
	if m.statusMsg != "" {
		if m.statusIsError {
			return dangerStyle.Render("⚠ " + m.statusMsg)
		}
		return successStyle.Render("✓ " + m.statusMsg)
	}

	help := "[↑/↓] Select  [Enter] Info  [Space] Folders  [Tab] Filters  [/] Search  [s] Sort  [o] Browser  [d] Delete  [q] Quit"
	if len(help) > m.width {
		help = "[↑/↓] Select  [Space] Folders  [Tab] Filters  [/] Search  [s] Sort  [q] Quit"
	}
	if len(help) > m.width {
		help = "[↑/↓] Select  [Tab] Filters  [q] Quit"
	}
	return dimStyle.Render(help)
}

func formatNumber(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	if n < 1000000 {
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	}
	return fmt.Sprintf("%.1fM", float64(n)/1000000)
}
