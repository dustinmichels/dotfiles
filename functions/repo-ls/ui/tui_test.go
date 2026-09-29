package ui

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/dustinmichels/repo-ls/models"
	"github.com/muesli/termenv"
)

func makeTestRepos() []*models.Repo {
	now := time.Now()

	return []*models.Repo{
		{
			Path:               "/root/project-a",
			RelPath:            "project-a",
			Name:               "project-a",
			Branch:             "main",
			HasUncommitted:     true,
			UncommittedCount:   2,
			UncommittedDetails: []string{" M file1.go", "?? file2.txt"},
			IsGitHub:           true,
			GitHubSlug:         "user/project-a",
			GitHubVisibility:   models.VisibilityPublic,
			GitHubWebURL:       "https://github.com/user/project-a",
			DateCreated:        now.Add(-30 * 24 * time.Hour),
			LastModified:       now.Add(-10 * time.Minute),
			LastCommit: &models.CommitInfo{
				Hash:     "aaa1111",
				Author:   "Alice",
				Date:     now.Add(-10 * time.Minute),
				Relative: "10m ago",
				Message:  "Initial commit",
			},
		},
		{
			Path:             "/root/nested/project-b",
			RelPath:          "nested/project-b",
			Name:             "project-b",
			Branch:           "feature/xyz",
			HasUncommitted:   false,
			IsGitHub:         true,
			GitHubSlug:       "user/project-b",
			GitHubVisibility: models.VisibilityPrivate,
			GitHubWebURL:     "https://github.com/user/project-b",
			DateCreated:      now.Add(-10 * 24 * time.Hour),
			LastModified:     now.Add(-2 * time.Hour),
			LastCommit: &models.CommitInfo{
				Hash:     "bbb2222",
				Author:   "Bob",
				Date:     now.Add(-2 * time.Hour),
				Relative: "2h ago",
				Message:  "Feature work",
			},
		},
		{
			Path:             "/root/local-c",
			RelPath:          "local-c",
			Name:             "local-c",
			Branch:           "master",
			HasUncommitted:   false,
			IsGitHub:         false,
			GitHubVisibility: models.VisibilityLocal,
			DateCreated:      now.Add(-60 * 24 * time.Hour),
			LastModified:     now.Add(-24 * time.Hour),
			LastCommit: &models.CommitInfo{
				Hash:     "ccc3333",
				Author:   "Charlie",
				Date:     now.Add(-24 * time.Hour),
				Relative: "1d ago",
				Message:  "Local repo only",
			},
		},
	}
}

func TestFilterModes(t *testing.T) {
	repos := makeTestRepos()
	m := NewModel("/root", repos)

	if len(m.filtered) != 3 {
		t.Fatalf("expected 3 repos initially, got %d", len(m.filtered))
	}

	// Filter Dirty
	m.filterMode = models.FilterUncommitted
	m.applyFiltersAndSort()
	if len(m.filtered) != 1 || m.filtered[0].Name != "project-a" {
		t.Fatalf("expected only project-a for Dirty filter, got %d", len(m.filtered))
	}

	// Filter Clean
	m.filterMode = models.FilterClean
	m.applyFiltersAndSort()
	if len(m.filtered) != 2 {
		t.Fatalf("expected 2 clean repos, got %d", len(m.filtered))
	}

	// Filter GitHub
	m.filterMode = models.FilterGitHub
	m.applyFiltersAndSort()
	if len(m.filtered) != 2 {
		t.Fatalf("expected 2 GitHub repos, got %d", len(m.filtered))
	}

	// Filter Local
	m.filterMode = models.FilterLocal
	m.applyFiltersAndSort()
	if len(m.filtered) != 1 || m.filtered[0].Name != "local-c" {
		t.Fatalf("expected 1 local repo, got %d", len(m.filtered))
	}

	// Filter Public
	m.filterMode = models.FilterPublic
	m.applyFiltersAndSort()
	if len(m.filtered) != 1 || m.filtered[0].Name != "project-a" {
		t.Fatalf("expected 1 public repo, got %d", len(m.filtered))
	}

	// Filter Private
	m.filterMode = models.FilterPrivate
	m.applyFiltersAndSort()
	if len(m.filtered) != 1 || m.filtered[0].Name != "project-b" {
		t.Fatalf("expected 1 private repo, got %d", len(m.filtered))
	}
}

func TestSearchQuery(t *testing.T) {
	repos := makeTestRepos()
	m := NewModel("/root", repos)

	m.searchInput.SetValue("nested")
	m.applyFiltersAndSort()
	if len(m.filtered) != 1 || m.filtered[0].Name != "project-b" {
		t.Fatalf("expected project-b for search 'nested', got %d", len(m.filtered))
	}

	m.searchInput.SetValue("project-a")
	m.applyFiltersAndSort()
	if len(m.filtered) != 1 || m.filtered[0].Name != "project-a" {
		t.Fatalf("expected project-a for search 'project-a', got %d", len(m.filtered))
	}

	m.searchInput.SetValue("nonexistent")
	m.applyFiltersAndSort()
	if len(m.filtered) != 0 {
		t.Fatalf("expected 0 repos for nonexistent search, got %d", len(m.filtered))
	}
}

func TestSorting(t *testing.T) {
	repos := makeTestRepos()
	m := NewModel("/root", repos)

	// Sort Recent (newest commit first)
	m.sortMode = models.SortRecent
	m.applyFiltersAndSort()
	if m.filtered[0].Name != "project-a" || m.filtered[1].Name != "project-b" || m.filtered[2].Name != "local-c" {
		t.Errorf("unexpected sort order for SortRecent: %s, %s, %s",
			m.filtered[0].Name, m.filtered[1].Name, m.filtered[2].Name)
	}

	// Sort DirtyFirst
	m.sortMode = models.SortDirtyFirst
	m.applyFiltersAndSort()
	if m.filtered[0].Name != "project-a" {
		t.Errorf("expected dirty repo first, got %s", m.filtered[0].Name)
	}
}

func TestNavigationKeys(t *testing.T) {
	repos := makeTestRepos()
	m := NewModel("/root", repos)

	if m.cursor != 0 {
		t.Errorf("expected initial cursor 0, got %d", m.cursor)
	}

	// Press 'j' (down)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = updated.(Model)
	if m.cursor != 1 {
		t.Errorf("expected cursor 1 after 'j', got %d", m.cursor)
	}

	// Press 'k' (up)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m = updated.(Model)
	if m.cursor != 0 {
		t.Errorf("expected cursor 0 after 'k', got %d", m.cursor)
	}
}

func TestDeleteModalFlow(t *testing.T) {
	repos := makeTestRepos()
	m := NewModel("/root", repos)

	// Press 'd' to open delete modal
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m = updated.(Model)
	if !m.showDeleteModal {
		t.Fatalf("expected delete modal to be open")
	}
	if m.deleteStep != 0 {
		t.Fatalf("expected deleteStep 0, got %d", m.deleteStep)
	}

	// Cancel with esc
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.showDeleteModal {
		t.Fatalf("expected delete modal to be closed after Esc")
	}

	// Reopen with 'd' and select option '1' (Local)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	m = updated.(Model)

	if m.deleteStep != 1 {
		t.Fatalf("expected deleteStep 1 after choosing option 1, got %d", m.deleteStep)
	}
	if m.deleteChoice != 1 {
		t.Fatalf("expected deleteChoice 1, got %d", m.deleteChoice)
	}

	// Cancel confirmation with Esc
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.showDeleteModal {
		t.Fatalf("expected delete modal to be closed after Esc")
	}
}

func TestRenderView(t *testing.T) {
	repos := makeTestRepos()
	m := NewModel("/root", repos)
	m.width = 120
	m.height = 30

	view := m.View()
	t.Logf("view:\n%s", view)
	if !strings.Contains(view, "REPO-LS") {
		t.Errorf("expected view to contain REPO-LS title")
	}
	if !strings.Contains(view, "project-a") {
		t.Errorf("expected view to contain project-a")
	}
	if !strings.Contains(strings.ToLower(view), "public") {
		t.Errorf("expected view to contain public visibility")
	}

	// Select project-a by name to test inspector with public GitHub badge
	for idx, it := range m.items {
		if it.Repo != nil && it.Repo.Name == "project-a" {
			m.cursor = idx
			break
		}
	}
	viewProjectA := m.View()
	if !strings.Contains(viewProjectA, "PUBLIC") {
		t.Errorf("expected view for project-a to contain PUBLIC badge")
	}
}
func TestSortCreatedAndModified(t *testing.T) {
	repos := makeTestRepos()
	m := NewModel("/root", repos)

	// Sort Created (newest created first: project-b (-10d), project-a (-30d), local-c (-60d))
	m.sortMode = models.SortCreated
	m.applyFiltersAndSort()
	if m.filtered[0].Name != "project-b" || m.filtered[1].Name != "project-a" || m.filtered[2].Name != "local-c" {
		t.Errorf("unexpected sort order for SortCreated: %s, %s, %s",
			m.filtered[0].Name, m.filtered[1].Name, m.filtered[2].Name)
	}

	// Sort Modified (newest modified first: project-a (-10m), project-b (-2h), local-c (-24h))
	m.sortMode = models.SortModified
	m.applyFiltersAndSort()
	if m.filtered[0].Name != "project-a" || m.filtered[1].Name != "project-b" || m.filtered[2].Name != "local-c" {
		t.Errorf("unexpected sort order for SortModified: %s, %s, %s",
			m.filtered[0].Name, m.filtered[1].Name, m.filtered[2].Name)
	}
}

func TestParentRepoWithNestedChildren(t *testing.T) {
	now := time.Now()
	repos := []*models.Repo{
		{
			Path:         "/root/x",
			RelPath:      "x",
			Name:         "x",
			Branch:       "main",
			DateCreated:  now.Add(-10 * 24 * time.Hour),
			LastModified: now.Add(-1 * time.Hour),
		},
		{
			Path:         "/root/x/y",
			RelPath:      "x/y",
			Name:         "y",
			Branch:       "main",
			DateCreated:  now.Add(-5 * 24 * time.Hour),
			LastModified: now.Add(-30 * time.Minute),
		},
		{
			Path:         "/root/x/z",
			RelPath:      "x/z",
			Name:         "z",
			Branch:       "main",
			DateCreated:  now.Add(-2 * 24 * time.Hour),
			LastModified: now.Add(-15 * time.Minute),
		},
		{
			Path:         "/root/standalone",
			RelPath:      "standalone",
			Name:         "standalone",
			Branch:       "main",
			DateCreated:  now.Add(-20 * 24 * time.Hour),
			LastModified: now.Add(-2 * time.Hour),
		},
		// Pure folder: ARCHIVE containing repos, but no root repo ARCHIVE
		{
			Path:         "/root/ARCHIVE/old1",
			RelPath:      "ARCHIVE/old1",
			Name:         "old1",
			Branch:       "master",
			DateCreated:  now.Add(-100 * 24 * time.Hour),
			LastModified: now.Add(-50 * 24 * time.Hour),
		},
		{
			Path:         "/root/ARCHIVE/old2",
			RelPath:      "ARCHIVE/old2",
			Name:         "old2",
			Branch:       "master",
			DateCreated:  now.Add(-90 * 24 * time.Hour),
			LastModified: now.Add(-40 * 24 * time.Hour),
		},
	}

	m := NewModel("/root", repos)
	m.sortMode = models.SortPath
	m.applyFiltersAndSort()

	// 1. Verify parent repo x:
	// x must appear exactly once as a parent repo with HasChildren=true, not as ItemKindFolder
	xCount := 0
	for _, it := range m.items {
		if it.Repo != nil && it.Repo.Name == "x" {
			xCount++
			if !it.HasChildren {
				t.Errorf("expected repo x to have HasChildren=true")
			}
			if it.RepoCount != 2 {
				t.Errorf("expected repo x to have RepoCount=2, got %d", it.RepoCount)
			}
		}
		if it.Kind == ItemKindFolder && it.Folder == "x" {
			t.Errorf("repo x should not also render as an ItemKindFolder")
		}
	}
	if xCount != 1 {
		t.Errorf("expected repo x to be rendered exactly once, got %d times", xCount)
	}

	// 2. Verify pure folder ARCHIVE:
	// ARCHIVE must render as ItemKindFolder
	archiveFolderFound := false
	for _, it := range m.items {
		if it.Kind == ItemKindFolder && it.Folder == "ARCHIVE" {
			archiveFolderFound = true
			if it.RepoCount != 2 {
				t.Errorf("expected ARCHIVE folder to have RepoCount=2, got %d", it.RepoCount)
			}
		}
	}
	if !archiveFolderFound {
		t.Errorf("expected ARCHIVE to be rendered as ItemKindFolder")
	}

	// Total items when expanded: ARCHIVE (folder) + old1 + old2 + standalone + x (parent) + y + z = 7
	if len(m.items) != 7 {
		t.Fatalf("expected 7 items when all expanded, got %d", len(m.items))
	}

	// Collapse folder "ARCHIVE"
	m.collapsedFolders["ARCHIVE"] = true
	m.applyFiltersAndSort()

	// When ARCHIVE is collapsed, old1 and old2 must not be in items
	for _, it := range m.items {
		if it.Repo != nil && strings.HasPrefix(filepath.ToSlash(it.Repo.RelPath), "ARCHIVE/") {
			t.Errorf("expected ARCHIVE child %s to be hidden when collapsed", it.Repo.RelPath)
		}
	}
	if len(m.items) != 5 {
		t.Fatalf("expected 5 items when ARCHIVE collapsed, got %d", len(m.items))
	}

	// Collapse parent repo "x"
	m.collapsedFolders["x"] = true
	m.applyFiltersAndSort()

	// When x is collapsed, y and z must not be in items
	for _, it := range m.items {
		if it.Repo != nil && (it.Repo.Name == "y" || it.Repo.Name == "z") {
			t.Errorf("expected nested repo %s to be hidden when parent x is collapsed", it.Repo.Name)
		}
	}
	if len(m.items) != 3 {
		t.Fatalf("expected 3 items when both collapsed, got %d", len(m.items))
	}

	// Expand all
	m.collapsedFolders = make(map[string]bool)
	m.applyFiltersAndSort()
	if len(m.items) != 7 {
		t.Fatalf("expected 7 items after expanding all, got %d", len(m.items))
	}
}

func TestNoSelectionAndSummary(t *testing.T) {
	repos := makeTestRepos()
	m := NewModel("/root", repos)
	m.width = 120
	m.height = 30

	// Initially cursor is 0
	if m.cursor != 0 {
		t.Errorf("expected initial cursor 0, got %d", m.cursor)
	}

	// Press Esc to deselect
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.cursor != -1 {
		t.Errorf("expected cursor -1 after Esc, got %d", m.cursor)
	}

	// View should render ALL REPOSITORIES SUMMARY
	view := m.View()
	if !strings.Contains(view, "ALL REPOSITORIES SUMMARY") {
		t.Errorf("expected view to contain ALL REPOSITORIES SUMMARY when cursor is -1")
	}

	// Press 'j' (down) from -1 should select row 0
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = updated.(Model)
	if m.cursor != 0 {
		t.Errorf("expected cursor 0 after down from -1, got %d", m.cursor)
	}

	// Press 'k' (up) from 0 should deselect to -1
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m = updated.(Model)
	if m.cursor != -1 || !m.noSelection {
		t.Errorf("expected cursor -1 and noSelection true after up from 0, got %d, %v", m.cursor, m.noSelection)
	}

	// Change filter mode, verify noSelection and cursor -1 survive
	m.filterMode = models.FilterClean
	m.applyFiltersAndSort()
	if !m.noSelection || m.cursor != -1 {
		t.Errorf("expected noSelection=true and cursor=-1 after filter change, got noSelection=%v cursor=%d", m.noSelection, m.cursor)
	}
	if cur := m.currentRepo(); cur != nil {
		t.Errorf("expected currentRepo() == nil when noSelection=true")
	}
	viewAfterFilter := m.View()
	if !strings.Contains(viewAfterFilter, "ALL REPOSITORIES SUMMARY") {
		t.Errorf("expected view to contain ALL REPOSITORIES SUMMARY after filter change with no selection")
	}
}

func TestTabFilterCycling(t *testing.T) {
	repos := makeTestRepos()
	m := NewModel("/root", repos)

	if m.filterMode != models.FilterAll {
		t.Fatalf("expected initial filter FilterAll")
	}

	expectedOrder := []models.FilterMode{
		models.FilterUncommitted,
		models.FilterClean,
		models.FilterGitHub,
		models.FilterLocal,
		models.FilterPublic,
		models.FilterPrivate,
		models.FilterAll,
	}

	for i, expected := range expectedOrder {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
		m = updated.(Model)
		if m.filterMode != expected {
			t.Errorf("step %d: expected filter %s, got %s", i, expected, m.filterMode)
		}
	}

	// Shift+Tab backward
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	m = updated.(Model)
	if m.filterMode != models.FilterPrivate {
		t.Errorf("expected FilterPrivate after shift+tab from FilterAll, got %s", m.filterMode)
	}
}

func TestDeleteModalTab(t *testing.T) {
	repos := makeTestRepos()
	m := NewModel("/root", repos)

	// Open delete modal with 'd'
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m = updated.(Model)
	if !m.showDeleteModal {
		t.Fatalf("expected delete modal open")
	}

	// Press Tab to cycle option: 0 -> 1 -> 2 -> 3 -> 1
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)
	if m.deleteChoice != 1 {
		t.Errorf("expected deleteChoice 1 after Tab, got %d", m.deleteChoice)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)
	if m.deleteChoice != 2 {
		t.Errorf("expected deleteChoice 2 after Tab, got %d", m.deleteChoice)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)
	if m.deleteChoice != 3 {
		t.Errorf("expected deleteChoice 3 after Tab, got %d", m.deleteChoice)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)
	if m.deleteChoice != 1 {
		t.Errorf("expected deleteChoice 1 after cycling Tab, got %d", m.deleteChoice)
	}
}

func TestDirtyRepoRedDot(t *testing.T) {
	oldProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() {
		lipgloss.SetColorProfile(oldProfile)
	})

	repos := makeTestRepos()
	m := NewModel("/root", repos)
	m.width = 120
	m.height = 30

	// Move cursor away from dirty repo project-a
	for idx, it := range m.items {
		if it.Repo != nil && !it.Repo.HasUncommitted {
			m.cursor = idx
			break
		}
	}

	view := m.View()
	foundDirtyLine := false
	for _, l := range strings.Split(view, "\n") {
		if strings.Contains(l, "project-a") {
			foundDirtyLine = true
			if !strings.Contains(l, "●") {
				t.Errorf("expected dirty line to contain dot ●")
			}
			// Verify dot is colored red (#FF5F87 is 255;95;135 in ANSI TrueColor)
			redSeq := "38;2;255;95;135"
			if !strings.Contains(l, redSeq) {
				t.Errorf("expected red color escape on dirty line, got line: %q", l)
			}

			// Verify the name "project-a" itself is not inside the red dangerStyle
			nameIdx := strings.Index(l, "project-a")
			prefix := l[:nameIdx]
			lastM := strings.LastIndex(prefix, "m")
			if lastM > 0 {
				lastEscStart := strings.LastIndex(prefix[:lastM], "\x1b[")
				if lastEscStart >= 0 {
					lastSgr := prefix[lastEscStart : lastM+1]
					if strings.Contains(lastSgr, redSeq) {
						t.Errorf("project-a name is preceded by red SGR escape %q; row text should not be red", lastSgr)
					}
				}
			}
			break
		}
	}
	if !foundDirtyLine {
		t.Errorf("expected to find line with project-a")
	}

	// Verify table header contains DATE-CREATED and LAST-MODIFIED
	if !strings.Contains(view, "DATE-CREATED") || !strings.Contains(view, "LAST-MODIFIED") {
		t.Errorf("expected table header to contain DATE-CREATED and LAST-MODIFIED")
	}
}

func TestColumnAlignment(t *testing.T) {
	repos := makeTestRepos()
	m := NewModel("/root", repos)
	m.width = 120
	m.height = 30

	tableOutput := m.renderTable(80, 10)
	lines := strings.Split(tableOutput, "\n")
	if len(lines) < 4 {
		t.Fatalf("expected at least 4 lines of table output")
	}

	headerRunes := []rune(ansi.Strip(lines[0]))
	headerStr := string(headerRunes)
	branchCol := strings.Index(headerStr, "BRANCH")
	visCol := strings.Index(headerStr, "VISIBILITY")
	createdCol := strings.Index(headerStr, "DATE-CREATED")
	modifiedCol := strings.Index(headerStr, "LAST-MODIFIED")

	if branchCol < 0 || visCol < 0 || createdCol < 0 || modifiedCol < 0 {
		t.Fatalf("failed to find header columns in stripped header: %q", headerStr)
	}

	// Check each repo row: stripped line should have BRANCH, VISIBILITY, CREATED, MODIFIED start at the exact same rune columns
	for rowIdx := 2; rowIdx < len(lines); rowIdx++ {
		raw := lines[rowIdx]
		if strings.TrimSpace(raw) == "" || strings.Contains(raw, "📁") {
			continue
		}
		rowRunes := []rune(ansi.Strip(raw))
		if len(rowRunes) <= modifiedCol {
			t.Errorf("row %d too short: len %d, expected > %d", rowIdx, len(rowRunes), modifiedCol)
			continue
		}

		if rowRunes[branchCol] == ' ' {
			t.Errorf("row %d has space at BRANCH column %d: %q", rowIdx, branchCol, string(rowRunes))
		}
		if rowRunes[branchCol-1] != ' ' {
			t.Errorf("row %d does not have separator before BRANCH column %d: %q", rowIdx, branchCol, string(rowRunes))
		}
		if rowRunes[visCol] == ' ' {
			t.Errorf("row %d has space at VISIBILITY column %d: %q", rowIdx, visCol, string(rowRunes))
		}
		if rowRunes[createdCol] == ' ' {
			t.Errorf("row %d has space at DATE-CREATED column %d: %q", rowIdx, createdCol, string(rowRunes))
		}
		if rowRunes[modifiedCol] == ' ' {
			t.Errorf("row %d has space at LAST-MODIFIED column %d: %q", rowIdx, modifiedCol, string(rowRunes))
		}
	}
}


func TestParentFilteredOutFallback(t *testing.T) {
	now := time.Now()
	repos := []*models.Repo{
		{
			Path:           "/root/x",
			RelPath:        "x",
			Name:           "x",
			Branch:         "main",
			HasUncommitted: false, // clean
			DateCreated:    now.Add(-10 * 24 * time.Hour),
			LastModified:   now.Add(-1 * time.Hour),
		},
		{
			Path:           "/root/x/y",
			RelPath:        "x/y",
			Name:           "y",
			Branch:         "main",
			HasUncommitted: true, // dirty
			DateCreated:    now.Add(-5 * 24 * time.Hour),
			LastModified:   now.Add(-30 * time.Minute),
		},
	}

	m := NewModel("/root", repos)
	// Filter by dirty
	m.filterMode = models.FilterUncommitted
	m.applyFiltersAndSort()

	// x/y is dirty, x is clean (so x is not in filtered)
	// x should fall back to a folder header so x/y is not orphaned
	if len(m.items) != 2 {
		t.Fatalf("expected 2 items (folder x + child y), got %d", len(m.items))
	}
	if m.items[0].Kind != ItemKindFolder || m.items[0].Folder != "x" {
		t.Errorf("expected item 0 to be folder header x, got %+v", m.items[0])
	}
	if m.items[1].Kind != ItemKindRepo || m.items[1].Repo.Name != "y" || !m.items[1].IsChild {
		t.Errorf("expected item 1 to be child repo y, got %+v", m.items[1])
	}
}
func TestSearchNoMatchClearReturnsTokeiCmd(t *testing.T) {
	repos := makeTestRepos()
	m := NewModel("/root", repos)

	// Search for something that matches nothing
	m.searchInput.SetValue("zzz_no_match_here")
	m.applyFiltersAndSort()

	if len(m.filtered) != 0 {
		t.Fatalf("expected 0 filtered repos, got %d", len(m.filtered))
	}
	if !m.noSelection || m.cursor != -1 {
		t.Errorf("expected noSelection=true and cursor=-1 with 0 matches")
	}

	// Verify loadingTokei["__all_repos__"] is NOT stuck true
	if m.loadingTokei[allReposTokeiKey] {
		t.Errorf("loadingTokei for all repos should not be true during applyFiltersAndSort")
	}

	// Clear search
	m.searchInput.SetValue("")
	m.applyFiltersAndSort()

	// Deselect and verify loadAllTokeiCmd returns a runnable non-nil command
	m.noSelection = true
	m.cursor = -1
	cmd := m.loadAllTokeiCmd()
	if cmd == nil {
		t.Errorf("expected loadAllTokeiCmd() to return non-nil cmd after clearing empty search")
	}
}
