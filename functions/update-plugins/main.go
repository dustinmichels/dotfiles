package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
)

// Palette & Styles
var (
	primaryColor   = lipgloss.Color("#7D56F4") // Vibrant Claude purple
	accentColor    = lipgloss.Color("#A78BFA") // Soft lavender
	secondaryColor = lipgloss.Color("#4B5563") // Slate gray
	mutedColor     = lipgloss.Color("#6B7280") // Dim gray
	successColor   = lipgloss.Color("#10B981") // Emerald green
	errorColor     = lipgloss.Color("#EF4444") // Coral red
	warnColor      = lipgloss.Color("#F59E0B") // Amber
	infoColor      = lipgloss.Color("#38BDF8") // Sky blue

	// Badges
	stepBadge = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(primaryColor).
			Padding(0, 1)

	modeBadge = lipgloss.NewStyle().
			Bold(true).
			Padding(0, 1)

	// Text styles
	stepTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#F9FAFB"))

	commandNote = lipgloss.NewStyle().
			Foreground(mutedColor).
			Italic(true)

	pluginNameStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#E0E7FF"))

	successStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(successColor)

	errorStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(errorColor)

	warnStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(warnColor)

	// Containers
	headerBox = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(primaryColor).
			Padding(0, 2)

	summaryBox = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(primaryColor).
			Padding(0, 2)
)

type installedPluginsFile struct {
	Plugins map[string]json.RawMessage `json:"plugins"`
}

type pluginListItem struct {
	ID string `json:"id"`
}

type pluginStatus int

const (
	statusUpdated pluginStatus = iota
	statusUpToDate
	statusFailed
	statusDryRun
)

type pluginScopeEntry struct {
	Scope        string `json:"scope"`
	InstallPath  string `json:"installPath"`
	Version      string `json:"version"`
	GitCommitSha string `json:"gitCommitSha"`
}

type pluginResult struct {
	id      string
	rc      int
	elapsed int
	status  pluginStatus
}

func runCmd(name string, args []string, stdin io.Reader, dryRun bool) (int, int) {
	if dryRun {
		fmt.Printf("   %s %s %s\n",
			mutedColorTag("⚡ dry-run: would run:"),
			name,
			strings.Join(args, " "),
		)
		return 0, 0
	}

	tStart := time.Now()
	cmd := exec.Command(name, args...)
	cmd.Stdin = stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	tElapsed := int(time.Since(tStart).Seconds())
	rc := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			rc = exitErr.ExitCode()
		} else {
			rc = 1
		}
	}
	return rc, tElapsed
}

func parsePluginScopeEntries(raw json.RawMessage) []pluginScopeEntry {
	var list []pluginScopeEntry
	if err := json.Unmarshal(raw, &list); err == nil {
		return list
	}
	var single pluginScopeEntry
	if err := json.Unmarshal(raw, &single); err == nil {
		return []pluginScopeEntry{single}
	}
	return nil
}

func getPluginFingerprint(homeDir string, id string) string {
	pluginFile := filepath.Join(homeDir, ".claude", "plugins", "installed_plugins.json")
	data, err := os.ReadFile(pluginFile)
	if err != nil {
		return ""
	}
	var parsed installedPluginsFile
	if err := json.Unmarshal(data, &parsed); err != nil {
		return ""
	}
	raw, ok := parsed.Plugins[id]
	if !ok {
		return ""
	}
	entries := parsePluginScopeEntries(raw)
	var parts []string
	for _, e := range entries {
		parts = append(parts, fmt.Sprintf("%s|%s|%s|%s", e.Scope, e.Version, e.GitCommitSha, e.InstallPath))
	}
	sort.Strings(parts)
	return strings.Join(parts, ";")
}


func mutedColorTag(text string) string {
	return lipgloss.NewStyle().Foreground(mutedColor).Render(text)
}

func printStepHeader(step, total, title, cmdNote string) {
	badge := stepBadge.Render(fmt.Sprintf("STEP %s/%s", step, total))
	heading := stepTitle.Render(title)
	fmt.Printf("\n%s  %s\n", badge, heading)
	if cmdNote != "" {
		fmt.Printf("   %s %s\n\n", mutedColorTag("↳"), commandNote.Render(cmdNote))
	} else {
		fmt.Println()
	}
}

func getInstalledPluginIDs(homeDir string) []string {
	var pluginIDs []string

	// 1. Try ~/.claude/plugins/installed_plugins.json
	pluginFile := filepath.Join(homeDir, ".claude", "plugins", "installed_plugins.json")
	if data, err := os.ReadFile(pluginFile); err == nil {
		var parsed installedPluginsFile
		if err := json.Unmarshal(data, &parsed); err == nil && len(parsed.Plugins) > 0 {
			for id := range parsed.Plugins {
				pluginIDs = append(pluginIDs, id)
			}
			sort.Strings(pluginIDs)
			return pluginIDs
		}
	}

	// 2. Fallback to `claude plugin list --json`
	cmd := exec.Command("claude", "plugin", "list", "--json")
	cmd.Stdin = strings.NewReader("")
	out, err := cmd.Output()
	if err == nil {
		var list []pluginListItem
		if err := json.Unmarshal(out, &list); err == nil {
			for _, item := range list {
				if item.ID != "" {
					pluginIDs = append(pluginIDs, item.ID)
				}
			}
			sort.Strings(pluginIDs)
			return pluginIDs
		}
	}

	return pluginIDs
}

func showHelp() {
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(primaryColor).
		Padding(0, 1).
		Render("Claude Plugin Updater")

	sub := lipgloss.NewStyle().Foreground(mutedColor).Render("Refreshes marketplaces, updates plugins, and prunes unused dependencies.")

	usageHdr := lipgloss.NewStyle().Bold(true).Foreground(accentColor).Render("USAGE")
	flagsHdr := lipgloss.NewStyle().Bold(true).Foreground(accentColor).Render("FLAGS")
	exHdr := lipgloss.NewStyle().Bold(true).Foreground(accentColor).Render("EXAMPLES")

	flagName := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#E0E7FF"))
	flagDesc := lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF"))

	fmt.Println()
	fmt.Printf("%s\n%s\n\n", title, sub)
	fmt.Printf("%s\n  update-plugins [flags]\n\n", usageHdr)
	fmt.Printf("%s\n", flagsHdr)
	fmt.Printf("  %s  %s\n", flagName.Render("-y, --yes    "), flagDesc.Render("Auto-accept prompts during updates and pruning"))
	fmt.Printf("  %s  %s\n", flagName.Render("-d, --dry-run"), flagDesc.Render("Simulate the process without executing claude commands"))
	fmt.Printf("  %s  %s\n\n", flagName.Render("-h, --help   "), flagDesc.Render("Show this help message"))
	fmt.Printf("%s\n", exHdr)
	fmt.Printf("  %s %s\n", lipgloss.NewStyle().Foreground(primaryColor).Render("•"), "update-plugins             # Interactive mode")
	fmt.Printf("  %s %s\n", lipgloss.NewStyle().Foreground(primaryColor).Render("•"), "update-plugins -y          # Non-interactive auto-confirm")
	fmt.Printf("  %s %s\n\n", lipgloss.NewStyle().Foreground(primaryColor).Render("•"), "update-plugins --dry-run   # Preview actions")
}

func main() {
	yesFlag := false
	dryRunFlag := false

	for _, arg := range os.Args[1:] {
		switch arg {
		case "-y", "--yes":
			yesFlag = true
		case "-d", "--dry-run":
			dryRunFlag = true
		case "-h", "--help":
			showHelp()
			return
		}
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = os.Getenv("HOME")
	}

	// Application Header Banner
	bannerTitle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).Render("⚡ Claude Plugin Updater")
	bannerDesc := lipgloss.NewStyle().Foreground(mutedColor).Render("Sync marketplaces, update installed plugins & prune cache")

	var badges []string
	if yesFlag {
		badges = append(badges, modeBadge.Copy().Background(infoColor).Foreground(lipgloss.Color("#000000")).Render("AUTO-ACCEPT: ON"))
	} else {
		badges = append(badges, modeBadge.Copy().Background(secondaryColor).Foreground(lipgloss.Color("#E5E7EB")).Render("INTERACTIVE"))
	}
	if dryRunFlag {
		badges = append(badges, modeBadge.Copy().Background(warnColor).Foreground(lipgloss.Color("#000000")).Render("DRY RUN"))
	}

	headerContent := fmt.Sprintf("%s  %s\n%s",
		bannerTitle,
		strings.Join(badges, " "),
		bannerDesc,
	)
	fmt.Println()
	fmt.Println(headerBox.Render(headerContent))

	startTime := time.Now()

	// [1/3] Updating marketplace catalogs...
	printStepHeader("1", "3", "Updating marketplace catalogs...", "claude plugin marketplace update </dev/null")
	rc, tElapsed := runCmd("claude", []string{"plugin", "marketplace", "update"}, strings.NewReader(""), dryRunFlag)
	if rc == 0 {
		fmt.Printf("   %s Marketplaces refreshed %s\n",
			successStyle.Render("✔"),
			mutedColorTag(fmt.Sprintf("(%ds)", tElapsed)),
		)
	} else {
		fmt.Printf("   %s Marketplaces update failed %s\n",
			errorStyle.Render("✖"),
			mutedColorTag(fmt.Sprintf("(exit %d, %ds)", rc, tElapsed)),
		)
	}

	// [2/3] Checking installed plugins...
	printStepHeader("2", "3", "Checking installed plugins...", "claude plugin update <id>")

	pluginIDs := getInstalledPluginIDs(homeDir)
	total := len(pluginIDs)
	if total == 0 {
		warnCard := lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(warnColor).
			Padding(0, 1).
			Render(fmt.Sprintf("%s No installed plugins found in %s or claude CLI.", warnStyle.Render("▲ Warning:"), homeDir))
		fmt.Println(warnCard)
		os.Exit(1)
	}

	fmt.Printf("   Found %s installed plugins:\n", lipgloss.NewStyle().Bold(true).Foreground(accentColor).Render(fmt.Sprintf("%d", total)))
	for i, id := range pluginIDs {
		bullet := lipgloss.NewStyle().Foreground(primaryColor).Render("•")
		num := mutedColorTag(fmt.Sprintf("%d.", i+1))
		fmt.Printf("     %s %s %s\n", bullet, num, pluginNameStyle.Render(id))
	}
	fmt.Println()

	var results []pluginResult
	var updatedCount int
	var upToDateCount int
	var failedCount int

	divider := lipgloss.NewStyle().Foreground(lipgloss.Color("#374151")).Render("   ───────────────────────────────────────────────────")

	for idx, id := range pluginIDs {
		fmt.Println(divider)
		prefix := lipgloss.NewStyle().Bold(true).Foreground(accentColor).Render(fmt.Sprintf("   [%d/%d]", idx+1, total))
		fmt.Printf("%s Updating %s\n", prefix, pluginNameStyle.Render(id))

		args := []string{"plugin", "update", id}
		if yesFlag {
			args = append(args, "-y")
		}

		beforeFP := ""
		if !dryRunFlag {
			beforeFP = getPluginFingerprint(homeDir, id)
		}

		rc, tElapsed := runCmd("claude", args, os.Stdin, dryRunFlag)

		var status pluginStatus
		if dryRunFlag {
			status = statusDryRun
			fmt.Printf("   %s %s %s\n",
				lipgloss.NewStyle().Foreground(warnColor).Render("⚡"),
				pluginNameStyle.Render(id),
				mutedColorTag("simulated"),
			)
		} else if rc != 0 {
			failedCount++
			status = statusFailed
			fmt.Printf("   %s %s %s\n",
				errorStyle.Render("✖"),
				pluginNameStyle.Render(id),
				errorStyle.Render(fmt.Sprintf("failed (exit %d in %ds)", rc, tElapsed)),
			)
		} else {
			afterFP := getPluginFingerprint(homeDir, id)
			isUpToDate := false
			if beforeFP != "" && afterFP != "" {
				isUpToDate = (beforeFP == afterFP)
			}
			if isUpToDate {
				upToDateCount++
				status = statusUpToDate
				fmt.Printf("   %s %s %s\n",
					lipgloss.NewStyle().Foreground(infoColor).Render("✔"),
					pluginNameStyle.Render(id),
					mutedColorTag(fmt.Sprintf("already up-to-date (%ds)", tElapsed)),
				)
			} else {
				updatedCount++
				status = statusUpdated
				fmt.Printf("   %s %s %s\n",
					successStyle.Render("✔"),
					pluginNameStyle.Render(id),
					mutedColorTag(fmt.Sprintf("updated in %ds", tElapsed)),
				)
			}
		}

		results = append(results, pluginResult{id: id, rc: rc, elapsed: tElapsed, status: status})
	}
	fmt.Println(divider)

	// [3/3] Pruning unused dependencies...
	printStepHeader("3", "3", "Pruning unused dependencies...", "claude plugin prune")
	pruneArgs := []string{"plugin", "prune"}
	if yesFlag {
		pruneArgs = append(pruneArgs, "-y")
	}

	rc, tElapsed = runCmd("claude", pruneArgs, os.Stdin, dryRunFlag)
	if rc == 0 {
		fmt.Printf("   %s Prune completed %s\n\n",
			successStyle.Render("✔"),
			mutedColorTag(fmt.Sprintf("(%ds)", tElapsed)),
		)
	} else {
		fmt.Printf("   %s Prune failed %s\n\n",
			errorStyle.Render("✖"),
			mutedColorTag(fmt.Sprintf("(exit %d, %ds)", rc, tElapsed)),
		)
	}

	// Summary Table
	summaryHeader := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).Render("Plugin Results Summary")
	fmt.Printf("   %s\n", summaryHeader)

	tbl := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(secondaryColor)).
		Headers("#", "PLUGIN", "STATUS", "DURATION")

	for i, r := range results {
		var statusText string
		switch r.status {
		case statusUpToDate:
			statusText = lipgloss.NewStyle().Foreground(infoColor).Render("✔ Already up-to-date")
		case statusUpdated:
			statusText = successStyle.Render("✔ Updated")
		case statusFailed:
			statusText = errorStyle.Render(fmt.Sprintf("✖ Failed (%d)", r.rc))
		case statusDryRun:
			statusText = lipgloss.NewStyle().Foreground(warnColor).Render("⚡ Dry run")
		}
		tbl.Row(
			fmt.Sprintf("%d", i+1),
			r.id,
			statusText,
			fmt.Sprintf("%ds", r.elapsed),
		)
	}

	tbl.StyleFunc(func(row, col int) lipgloss.Style {
		if row == table.HeaderRow {
			return lipgloss.NewStyle().
				Bold(true).
				Foreground(accentColor).
				Padding(0, 1)
		}
		base := lipgloss.NewStyle().Padding(0, 1)
		switch col {
		case 0:
			return base.Foreground(mutedColor)
		case 1:
			return base.Foreground(lipgloss.Color("#F3F4F6")).Bold(true)
		case 3:
			return base.Foreground(mutedColor)
		default:
			return base
		}
	})

	// Indent table slightly
	tableRendered := tbl.Render()
	for _, line := range strings.Split(tableRendered, "\n") {
		fmt.Printf("   %s\n", line)
	}

	// Overall Summary Box
	totalElapsed := int(time.Since(startTime).Seconds())
	var summaryText strings.Builder

	if dryRunFlag {
		summaryText.WriteString(fmt.Sprintf("%s Dry run completed for %d plugins %s\n",
			lipgloss.NewStyle().Foreground(warnColor).Render("⚡"),
			total,
			mutedColorTag(fmt.Sprintf("(Total time: %ds)", totalElapsed)),
		))
	} else if failedCount == 0 {
		if updatedCount == 0 {
			summaryText.WriteString(fmt.Sprintf("%s All %d plugins are already up-to-date! %s\n",
				successStyle.Render("✔"),
				total,
				mutedColorTag(fmt.Sprintf("(Total time: %ds)", totalElapsed)),
			))
		} else if upToDateCount == 0 {
			summaryText.WriteString(fmt.Sprintf("%s All %d plugins updated successfully! %s\n",
				successStyle.Render("✔"),
				total,
				mutedColorTag(fmt.Sprintf("(Total time: %ds)", totalElapsed)),
			))
		} else {
			summaryText.WriteString(fmt.Sprintf("%s %d updated, %d already up-to-date %s\n",
				successStyle.Render("✔"),
				updatedCount,
				upToDateCount,
				mutedColorTag(fmt.Sprintf("(Total time: %ds)", totalElapsed)),
			))
		}
	} else {
		summaryText.WriteString(fmt.Sprintf("%s %d updated, %d already up-to-date, %d failed %s\n",
			errorStyle.Render("▲"),
			updatedCount,
			upToDateCount,
			failedCount,
			mutedColorTag(fmt.Sprintf("(Total time: %ds)", totalElapsed)),
		))
	}
	summaryText.WriteString(mutedColorTag("Tip: Restart Claude or run /reload-plugins in omp to activate changes."))

	fmt.Println()
	fmt.Println(summaryBox.Render(summaryText.String()))
	fmt.Println()
}
