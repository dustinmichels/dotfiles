package main

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestSortingByDate(t *testing.T) {
	now := time.Now()
	items := []PackageItem{
		{Name: "cask-old", Type: "cask", Epoch: now.Add(-4 * time.Hour).Unix(), InstalledAt: now.Add(-4 * time.Hour)},
		{Name: "formula-new", Type: "formula", Epoch: now.Add(-1 * time.Hour).Unix(), InstalledAt: now.Add(-1 * time.Hour)},
		{Name: "cask-new", Type: "cask", Epoch: now.Add(-2 * time.Hour).Unix(), InstalledAt: now.Add(-2 * time.Hour)},
		{Name: "formula-old", Type: "formula", Epoch: now.Add(-3 * time.Hour).Unix(), InstalledAt: now.Add(-3 * time.Hour)},
	}

	m := initialModel(items, FilterAll)
	if len(m.filtered) != 4 {
		t.Fatalf("expected 4 items, got %d", len(m.filtered))
	}

	// Sorted purely by date descending: newest first
	expectedOrder := []string{"formula-new", "cask-new", "formula-old", "cask-old"}
	for i, expected := range expectedOrder {
		if m.filtered[i].Name != expected {
			t.Errorf("at index %d: expected %s, got %s", i, expected, m.filtered[i].Name)
		}
	}
}

func TestCaskColorInRenderTableView(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	now := time.Now()
	items := []PackageItem{
		{Name: "formula-a", Type: "formula", Epoch: now.Add(-1 * time.Hour).Unix(), InstalledAt: now.Add(-1 * time.Hour)},
		{Name: "cask-a", Type: "cask", Epoch: now.Add(-2 * time.Hour).Unix(), InstalledAt: now.Add(-2 * time.Hour)},
	}

	m := initialModel(items, FilterAll)
	view := m.renderTableView()

	lines := strings.Split(view, "\n")
	if len(lines) < 4 {
		t.Fatalf("expected at least 4 lines in rendered table view, got %d", len(lines))
	}

	// Line 0: Header
	// Line 1: Border
	// Line 2: formula-a (selected, so tableSelectedStyle)
	// Line 3: cask-a (unselected, should have caskColor)
	caskLine := lines[3]
	if !strings.Contains(caskLine, "cask-a") {
		t.Fatalf("expected line 3 to contain cask-a, got %q", caskLine)
	}
	if !strings.Contains(caskLine, "\x1b[38;2;236;72;153m") {
		t.Errorf("expected line 3 to be styled with caskColor (#EC4899), got: %q", caskLine)
	}
}

func TestDeleteConfirmationWithX(t *testing.T) {
	items := []PackageItem{
		{Name: "test-pkg", Type: "formula", Epoch: 100},
	}
	m := initialModel(items, FilterAll)

	// 1. Press 'x' on homepage table: should enter confirmation modal
	newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	m = newM.(model)
	if cmd != nil {
		t.Fatal("expected nil cmd on first 'x'")
	}
	if !m.showDetails || m.modalAction != actionConfirmDelete {
		t.Fatalf("expected showDetails=true and modalAction=actionConfirmDelete, got details=%v action=%v", m.showDetails, m.modalAction)
	}

	// 2. Pressing 'y' should NOT trigger delete
	newM, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = newM.(model)
	if cmd != nil {
		t.Fatal("expected nil cmd when pressing 'y'")
	}
	if !m.showDetails || m.modalAction != actionConfirmDelete {
		t.Fatalf("expected still in confirm modal after 'y'")
	}

	// 3. Pressing 'x' a second time should trigger delete cmd
	newM, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	m = newM.(model)
	if cmd == nil {
		t.Fatal("expected non-nil cmd when confirming with 'x'")
	}

	// 4. brewFinishedMsg after uninstall should return to homepage (showDetails=false, search cleared)
	m.searchInput.SetValue("test-pkg")
	m.searching = true
	newM, _ = m.Update(brewFinishedMsg{action: "uninstall", itemName: "test-pkg", err: nil})
	m = newM.(model)

	if m.showDetails {
		t.Errorf("expected showDetails=false (homepage) after delete")
	}
	if m.modalAction != actionNone {
		t.Errorf("expected modalAction=actionNone after delete")
	}
	if m.selectedItem != nil {
		t.Errorf("expected selectedItem=nil after delete")
	}
	if m.searchInput.Value() != "" || m.searching {
		t.Errorf("expected search reset to empty on homepage after delete")
	}
}

func TestDeleteCancellationReturnsToHomepage(t *testing.T) {
	items := []PackageItem{
		{Name: "test-pkg", Type: "formula", Epoch: 100},
	}
	m := initialModel(items, FilterAll)

	// Press 'x' to enter confirm modal
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	m = newM.(model)

	// Press 'esc' to cancel -> should return to homepage
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = newM.(model)

	if m.showDetails {
		t.Errorf("expected showDetails=false after cancelling delete")
	}
	if m.modalAction != actionNone {
		t.Errorf("expected modalAction=actionNone after cancelling delete")
	}
}
