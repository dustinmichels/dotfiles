package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bitfield/script"
	"github.com/charmbracelet/huh"
)

func currentDirName() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return filepath.Base(cwd)
}

func main() {
	var (
		repoName      = currentDirName()
		visibility    string
		connectRemote bool
	)

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Repository name").
				Description("Press enter to use current directory name").
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
	)

	if err := form.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	fmt.Printf("\nCreating %s repo: %s\n", visibility, repoName)
	if _, err := script.Exec(fmt.Sprintf("gh repo create %s --%s", repoName, visibility)).Stdout(); err != nil {
		fmt.Fprintf(os.Stderr, "gh repo create failed: %v\n", err)
		os.Exit(1)
	}

	if connectRemote {
		username, err := script.Exec("gh api user --jq .login").String()
		if err != nil {
			fmt.Fprintf(os.Stderr, "could not get GitHub username: %v\n", err)
			os.Exit(1)
		}
		username = strings.TrimSpace(username)

		remoteURL := fmt.Sprintf("https://github.com/%s/%s", username, repoName)
		if _, err := script.Exec(fmt.Sprintf("git remote add origin %s", remoteURL)).Stdout(); err != nil {
			fmt.Fprintf(os.Stderr, "git remote add failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Remote 'origin' → %s\n", remoteURL)
	}
}
