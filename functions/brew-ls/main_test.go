package main

import (
	"strings"
	"testing"
	"time"

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
