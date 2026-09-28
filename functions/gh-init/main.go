package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/bitfield/script"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

func currentDirName() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return filepath.Base(cwd)
}
func checkGitRepo() (string, string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "Not a Git Repository", "The current directory is not a git repository.\nRun 'git init' first before running gh-init.", errors.New("not a git repository")
	}
	toplevel := strings.TrimSpace(string(out))
	cwd, err := os.Getwd()
	if err == nil {
		realCwd, err1 := filepath.EvalSymlinks(cwd)
		realTop, err2 := filepath.EvalSymlinks(toplevel)
		if err1 == nil && err2 == nil && realCwd != realTop {
			return "Not at Repository Root", fmt.Sprintf("The current directory is inside a git repository, but not at the root.\nRepository root is: %s\nPlease run gh-init from the root of the repository.", toplevel), errors.New("not at repository root")
		}
	}
	return "", "", nil
}

func getGitHubUsername() string {
	out, err := exec.Command("gh", "api", "user", "--jq", ".login").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
const (
	formWidth = 42
	gap       = 2
)

type previewModel struct {
	form          *huh.Form
	repoName      *string
	visibility    *string
	connectRemote *bool
	username      string
	width         int
	quitting      bool
	aborted       bool
}
func (m previewModel) Init() tea.Cmd {
	return m.form.Init()
}

func (m previewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.aborted = true
			m.quitting = true
			return m, tea.Quit
		}
	}
	form, cmd := m.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.form = f
	}

	if m.form.State == huh.StateCompleted {
		m.quitting = true
		return m, tea.Quit
	}
	if m.form.State == huh.StateAborted {
		m.aborted = true
		m.quitting = true
		return m, tea.Quit
	}

	return m, cmd
}

func (m previewModel) View() string {
	if m.quitting {
		return ""
	}

	repo := strings.TrimSpace(*m.repoName)
	if repo == "" {
		repo = currentDirName()
	}
	if repo == "" {
		repo = "<repo-name>"
	}
	vis := *m.visibility
	if vis == "" {
		vis = "private"
	}

	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("242")).Render
	cmdName := lipgloss.NewStyle().Foreground(lipgloss.Color("75")).Bold(true).Render
	arg := lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Render
	flag := lipgloss.NewStyle().Foreground(lipgloss.Color("141")).Render

	c1 := fmt.Sprintf("%s %s %s %s", dim("$"), cmdName("gh repo create"), arg(repo), flag("--"+vis))
	previewBody := c1

	if *m.connectRemote {
		owner := m.username
		if owner == "" {
			owner = "<owner>"
		}
		c2 := fmt.Sprintf("%s %s %s %s", dim("$"), cmdName("git remote add"), arg("origin"), arg(fmt.Sprintf("https://github.com/%s/%s", owner, repo)))
		previewBody += "\n" + c2
	}

	header := lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true).Render("Commands that will run")

	left := lipgloss.NewStyle().Width(formWidth).MaxWidth(formWidth).Render(m.form.View())
	leftW := lipgloss.Width(left)

	stacked := m.width > 0 && m.width < 75
	boxAvail := m.width - leftW - gap
	if stacked {
		boxAvail = m.width
	} else if m.width == 0 {
		boxAvail = 80
	}

	rawContent := header + "\n\n" + previewBody
	contentW := lipgloss.Width(rawContent)
	w := contentW + 2
	if w > boxAvail-2 {
		w = boxAvail - 2
	}
	if w < 24 {
		w = 24
	}

	previewBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(0, 1).
		Width(w).
		Render(rawContent)

	if stacked {
		return lipgloss.JoinVertical(lipgloss.Left, left, "", previewBox)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, left, strings.Repeat(" ", gap), previewBox)
}

func main() {
	if title, desc, warnErr := checkGitRepo(); warnErr != nil {
		note := huh.NewNote().
			Title(title).
			Description(desc).
			Next(true).
			NextLabel("Quit")

		_ = huh.NewForm(huh.NewGroup(note)).Run()
		os.Exit(0)
	}

	var (
		repoName      = currentDirName()
		visibility    = "private"
		connectRemote = true
		username      = getGitHubUsername()
	)

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Repository name").
				Description("Default: current directory").
				CharLimit(100).
				Value(&repoName),

			huh.NewSelect[string]().
				Title("Visibility").
				Options(
					huh.NewOption("Private", "private"),
					huh.NewOption("Public", "public"),
				).
				Value(&visibility),

			huh.NewConfirm().
				Title("Connect current directory to remote?").
				Value(&connectRemote),
		),
	).WithWidth(formWidth)
	m := previewModel{
		form:          form,
		repoName:      &repoName,
		visibility:    &visibility,
		connectRemote: &connectRemote,
		username:      username,
	}
	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		if errors.Is(err, tea.ErrInterrupted) {
			os.Exit(0)
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	mdl, ok := finalModel.(previewModel)
	if !ok || mdl.aborted || mdl.form.State != huh.StateCompleted {
		os.Exit(0)
	}

	repoName = strings.TrimSpace(repoName)
	if repoName == "" {
		repoName = currentDirName()
	}

	fmt.Printf("\nCreating %s repo: %s\n", visibility, repoName)
	if _, err := script.Exec(fmt.Sprintf("gh repo create %s --%s", repoName, visibility)).Stdout(); err != nil {
		fmt.Fprintf(os.Stderr, "gh repo create failed: %v\n", err)
		os.Exit(1)
	}

	if connectRemote {
		if username == "" {
			username = getGitHubUsername()
			if username == "" {
				fmt.Fprintln(os.Stderr, "could not get GitHub username from gh api")
				os.Exit(1)
			}
		}

		remoteURL := fmt.Sprintf("https://github.com/%s/%s", username, repoName)
		if _, err := script.Exec(fmt.Sprintf("git remote add origin %s", remoteURL)).Stdout(); err != nil {
			fmt.Fprintf(os.Stderr, "git remote add failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Remote 'origin' → %s\n", remoteURL)
	}
}
