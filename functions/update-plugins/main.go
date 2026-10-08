package main

import (
	"fmt"
	"os"
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

type pluginStatus int

const (
	statusUpdated pluginStatus = iota
	statusUpToDate
	statusFailed
	statusDryRun
	statusSkipped
)

type pluginResult struct {
	id      string
	rc      int
	elapsed int
	status  pluginStatus
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

func showHelp() {
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(primaryColor).
		Padding(0, 1).
		Render("Unified Agent Updater")

	sub := lipgloss.NewStyle().Foreground(mutedColor).Render("Refreshes marketplaces, updates plugins, prunes cache & reconciles skills across Claude, Codex, OMP, Pi & Gemini.")

	usageHdr := lipgloss.NewStyle().Bold(true).Foreground(accentColor).Render("USAGE")
	flagsHdr := lipgloss.NewStyle().Bold(true).Foreground(accentColor).Render("FLAGS")
	exHdr := lipgloss.NewStyle().Bold(true).Foreground(accentColor).Render("EXAMPLES")

	flagName := lipgloss.NewStyle().Foreground(lipgloss.Color("#F3F4F6")).Bold(true)
	flagDesc := lipgloss.NewStyle().Foreground(mutedColor)

	fmt.Printf("%s\n%s\n\n", title, sub)
	fmt.Printf("%s\n  update-plugins [flags]\n\n", usageHdr)
	fmt.Printf("%s\n", flagsHdr)
	fmt.Printf("  %s  %s\n", flagName.Render("-y, --yes         "), flagDesc.Render("Auto-accept confirmation prompts"))
	fmt.Printf("  %s  %s\n", flagName.Render("-d, --dry-run     "), flagDesc.Render("Simulate execution without running mutation commands"))
	fmt.Printf("  %s  %s\n", flagName.Render("    --skills-only "), flagDesc.Render("Run only skill phases (Phases 1-4)"))
	fmt.Printf("  %s  %s\n", flagName.Render("    --plugins-only"), flagDesc.Render("Run only plugin & marketplace phases (Phases 5-7)"))
	fmt.Printf("  %s  %s\n", flagName.Render("    --skip-prune  "), flagDesc.Render("Skip cache pruning step"))
	fmt.Printf("  %s  %s\n", flagName.Render("    --agents <list>"), flagDesc.Render("Comma-separated list of target agents (claude,codex,omp,gemini,pi)"))
	fmt.Printf("  %s  %s\n", flagName.Render("    --all-skills  "), flagDesc.Render("Alias to include all global skill checks"))
	fmt.Printf("  %s  %s\n\n", flagName.Render("-h, --help        "), flagDesc.Render("Show this help message"))
	fmt.Printf("%s\n", exHdr)
	fmt.Printf("  %s %s\n", lipgloss.NewStyle().Foreground(primaryColor).Render("•"), "update-plugins               # Update all agent plugins and skills")
	fmt.Printf("  %s %s\n", lipgloss.NewStyle().Foreground(primaryColor).Render("•"), "update-plugins -y            # Non-interactive auto-confirm")
	fmt.Printf("  %s %s\n", lipgloss.NewStyle().Foreground(primaryColor).Render("•"), "update-plugins --dry-run     # Preview execution actions")
	fmt.Printf("  %s %s\n", lipgloss.NewStyle().Foreground(primaryColor).Render("•"), "update-plugins --skills-only # Reconcile and update skills only")
	fmt.Printf("  %s %s\n", lipgloss.NewStyle().Foreground(primaryColor).Render("•"), "update-plugins --agents claude,omp # Target specific agents")
	fmt.Printf("  %s %s\n\n", lipgloss.NewStyle().Foreground(primaryColor).Render("•"), "update-plugins --skip-prune  # Keep older cached plugin versions")
}

func parseAgents(arg string) (map[string]bool, error) {
	valid := map[string]bool{
		"claude": true,
		"codex":  true,
		"omp":    true,
		"gemini": true,
		"pi":     true,
	}

	result := make(map[string]bool)
	parts := strings.Split(arg, ",")
	for _, p := range parts {
		name := strings.ToLower(strings.TrimSpace(p))
		if name == "" {
			continue
		}
		if !valid[name] {
			return nil, fmt.Errorf("unknown agent '%s' (valid: claude, codex, omp, gemini, pi)", name)
		}
		result[name] = true
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("no valid agents specified in --agents list")
	}
	return result, nil
}

func main() {
	yesFlag := false
	dryRunFlag := false
	skillsOnlyFlag := false
	pluginsOnlyFlag := false
	skipPruneFlag := false
	allSkillsFlag := false
	var agentsArg string

	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-y" || arg == "--yes":
			yesFlag = true
		case arg == "-d" || arg == "--dry-run":
			dryRunFlag = true
		case arg == "--skills-only":
			skillsOnlyFlag = true
		case arg == "--plugins-only":
			pluginsOnlyFlag = true
		case arg == "--skip-prune":
			skipPruneFlag = true
		case arg == "--all-skills":
			allSkillsFlag = true
		case arg == "--agents":
			if i+1 < len(args) {
				i++
				agentsArg = args[i]
			} else {
				fmt.Printf("%s %s\n", errorStyle.Render("✖"), "--agents requires a comma-separated list of agents")
				os.Exit(1)
			}
		case strings.HasPrefix(arg, "--agents="):
			agentsArg = strings.TrimPrefix(arg, "--agents=")
		case arg == "-h" || arg == "--help":
			showHelp()
			return
		default:
			fmt.Printf("%s Unknown flag: %s\n", errorStyle.Render("✖"), arg)
			showHelp()
			os.Exit(1)
		}
	}

	if skillsOnlyFlag && pluginsOnlyFlag {
		fmt.Printf("%s %s\n", errorStyle.Render("✖"), "Cannot specify both --skills-only and --plugins-only")
		os.Exit(1)
	}

	enabledAgents := map[string]bool{
		"claude": true,
		"codex":  true,
		"omp":    true,
		"gemini": true,
		"pi":     true,
	}

	if agentsArg != "" {
		parsed, err := parseAgents(agentsArg)
		if err != nil {
			fmt.Printf("%s %s\n", errorStyle.Render("✖"), err.Error())
			os.Exit(1)
		}
		enabledAgents = parsed
	}

	runSkills := !pluginsOnlyFlag
	runPlugins := !skillsOnlyFlag

	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = os.Getenv("HOME")
	}

	// Count total steps
	totalSteps := 0
	if runSkills {
		totalSteps += 4 // Phase 1, Phase 2, Phase 3, Phase 4
	}
	if runPlugins {
		totalSteps += 2 // Phase 5, Phase 6
		if !skipPruneFlag && enabledAgents["claude"] {
			totalSteps++ // Phase 7
		}
	}

	// Application Header Banner
	bannerTitle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).Render("⚡ Unified Agent Updater")
	var descParts []string
	if runSkills {
		descParts = append(descParts, "agent skills")
	}
	if runPlugins {
		descParts = append(descParts, "marketplaces & plugins")
	}
	bannerDesc := lipgloss.NewStyle().Foreground(mutedColor).Render(fmt.Sprintf("Update %s across connected agents", strings.Join(descParts, " and ")))

	var badges []string
	if yesFlag {
		badges = append(badges, modeBadge.Copy().Background(successColor).Foreground(lipgloss.Color("#FFFFFF")).Render("AUTO-CONFIRM"))
	}
	if dryRunFlag {
		badges = append(badges, modeBadge.Copy().Background(warnColor).Foreground(lipgloss.Color("#000000")).Render("DRY RUN"))
	}
	if skillsOnlyFlag {
		badges = append(badges, modeBadge.Copy().Background(secondaryColor).Foreground(lipgloss.Color("#E5E7EB")).Render("SKILLS ONLY"))
	}
	if pluginsOnlyFlag {
		badges = append(badges, modeBadge.Copy().Background(secondaryColor).Foreground(lipgloss.Color("#E5E7EB")).Render("PLUGINS ONLY"))
	}
	if skipPruneFlag {
		badges = append(badges, modeBadge.Copy().Background(secondaryColor).Foreground(lipgloss.Color("#E5E7EB")).Render("NO PRUNE"))
	}
	if allSkillsFlag {
		badges = append(badges, modeBadge.Copy().Background(primaryColor).Foreground(lipgloss.Color("#FFFFFF")).Render("ALL SKILLS"))
	}
	if agentsArg != "" {
		badges = append(badges, modeBadge.Copy().Background(primaryColor).Foreground(lipgloss.Color("#FFFFFF")).Render("AGENTS: "+agentsArg))
	}

	headerContent := fmt.Sprintf("%s  %s\n%s",
		bannerTitle,
		strings.Join(badges, " "),
		bannerDesc,
	)
	fmt.Println(headerBox.Render(headerContent))

	startTime := time.Now()
	currentStep := 1
	var results []pluginResult

	// ==========================================
	// Phase 1: Tool Binaries
	// ==========================================
	if runSkills {
		printStepHeader(fmt.Sprintf("%d", currentStep), fmt.Sprintf("%d", totalSteps),
			"Updating tool binaries...", "uv tool upgrade browser-use && browser-use --reload")
		currentStep++

		buRes, ok := runPhaseBinaries(homeDir, yesFlag, dryRunFlag)
		if ok {
			results = append(results, buRes)
			switch buRes.status {
			case statusDryRun:
				fmt.Printf("   %s %s %s\n",
					lipgloss.NewStyle().Foreground(warnColor).Render("⚡"),
					pluginNameStyle.Render(buRes.id),
					mutedColorTag("simulated"),
				)
			case statusFailed:
				fmt.Printf("   %s %s %s\n",
					errorStyle.Render("✖"),
					pluginNameStyle.Render(buRes.id),
					errorStyle.Render(fmt.Sprintf("failed (exit %d in %ds)", buRes.rc, buRes.elapsed)),
				)
			case statusUpToDate:
				fmt.Printf("   %s %s %s\n",
					lipgloss.NewStyle().Foreground(infoColor).Render("✔"),
					pluginNameStyle.Render(buRes.id),
					mutedColorTag(fmt.Sprintf("already up-to-date (%ds)", buRes.elapsed)),
				)
			case statusUpdated:
				fmt.Printf("   %s %s %s\n",
					successStyle.Render("✔"),
					pluginNameStyle.Render(buRes.id),
					mutedColorTag(fmt.Sprintf("updated in %ds", buRes.elapsed)),
				)
			}
		}
	}

	// ==========================================
	// Phase 2: Canonical Skills Hub
	// ==========================================
	if runSkills {
		printStepHeader(fmt.Sprintf("%d", currentStep), fmt.Sprintf("%d", totalSteps),
			"Updating canonical skills hub...", "npx skills update -g -y")
		currentStep++

		hubRes, ok := runPhaseCanonicalSkills(homeDir, dryRunFlag)
		if ok {
			results = append(results, hubRes)
			switch hubRes.status {
			case statusDryRun:
				fmt.Printf("   %s %s %s\n",
					lipgloss.NewStyle().Foreground(warnColor).Render("⚡"),
					pluginNameStyle.Render(hubRes.id),
					mutedColorTag("simulated"),
				)
			case statusFailed:
				fmt.Printf("   %s %s %s\n",
					errorStyle.Render("✖"),
					pluginNameStyle.Render(hubRes.id),
					errorStyle.Render(fmt.Sprintf("failed (exit %d in %ds)", hubRes.rc, hubRes.elapsed)),
				)
			default:
				fmt.Printf("   %s %s %s\n",
					successStyle.Render("✔"),
					pluginNameStyle.Render(hubRes.id),
					mutedColorTag(fmt.Sprintf("updated in %ds", hubRes.elapsed)),
				)
			}
		}
	}

	// ==========================================
	// Phase 3: Standalone Tool Skill Generation
	// ==========================================
	if runSkills {
		printStepHeader(fmt.Sprintf("%d", currentStep), fmt.Sprintf("%d", totalSteps),
			"Generating standalone tool skills...", "browser-use skill install --no-install")
		currentStep++

		buSkillRes, ok := runPhaseStandaloneSkills(homeDir, dryRunFlag)
		if ok {
			results = append(results, buSkillRes)
			switch buSkillRes.status {
			case statusDryRun:
				fmt.Printf("   %s %s %s\n",
					lipgloss.NewStyle().Foreground(warnColor).Render("⚡"),
					pluginNameStyle.Render(buSkillRes.id),
					mutedColorTag("simulated"),
				)
			case statusFailed:
				fmt.Printf("   %s %s %s\n",
					errorStyle.Render("✖"),
					pluginNameStyle.Render(buSkillRes.id),
					errorStyle.Render(fmt.Sprintf("failed (exit %d in %ds)", buSkillRes.rc, buSkillRes.elapsed)),
				)
			case statusUpToDate:
				fmt.Printf("   %s %s %s\n",
					lipgloss.NewStyle().Foreground(infoColor).Render("✔"),
					pluginNameStyle.Render(buSkillRes.id),
					mutedColorTag(fmt.Sprintf("already up-to-date (%ds)", buSkillRes.elapsed)),
				)
			case statusUpdated:
				fmt.Printf("   %s %s %s\n",
					successStyle.Render("✔"),
					pluginNameStyle.Render(buSkillRes.id),
					mutedColorTag(fmt.Sprintf("updated in %ds", buSkillRes.elapsed)),
				)
			}
		}
	}

	// ==========================================
	// Phase 4: Hub-and-Spoke Reconciliation
	// ==========================================
	if runSkills {
		printStepHeader(fmt.Sprintf("%d", currentStep), fmt.Sprintf("%d", totalSteps),
			"Reconciling hub-and-spoke skills...", "sync ~/.agents/skills -> ~/.claude, ~/.pi, ~/.gemini, ~/.omp")
		currentStep++

		reconRes, stats := runPhaseReconcile(homeDir, enabledAgents, dryRunFlag)
		results = append(results, reconRes)

		if stats.HealedCount > 0 {
			fmt.Printf("   %s Healed %d missing skill link(s): %s\n",
				successStyle.Render("✔"),
				stats.HealedCount,
				mutedColorTag(strings.Join(stats.HealedNames, ", ")),
			)
		}
		if stats.PrunedCount > 0 {
			fmt.Printf("   %s Pruned %d dangling skill link(s): %s\n",
				warnStyle.Render("✂"),
				stats.PrunedCount,
				mutedColorTag(strings.Join(stats.PrunedNames, ", ")),
			)
		}
		if stats.HealedCount == 0 && stats.PrunedCount == 0 {
			fmt.Printf("   %s All agent skill spokes are healthy and in sync\n",
				lipgloss.NewStyle().Foreground(infoColor).Render("✔"))
		}
	}

	// ==========================================
	// Phase 5: Remote Marketplaces
	// ==========================================
	if runPlugins {
		printStepHeader(fmt.Sprintf("%d", currentStep), fmt.Sprintf("%d", totalSteps),
			"Refreshing remote marketplaces...", "claude, codex, omp marketplace updates")
		currentStep++

		mpResults := runPhaseMarketplaces(homeDir, enabledAgents, dryRunFlag)
		for _, r := range mpResults {
			results = append(results, r)
			if r.status == statusFailed {
				fmt.Printf("   %s %s failed (exit %d in %ds)\n",
					errorStyle.Render("✖"), pluginNameStyle.Render(r.id), r.rc, r.elapsed)
			} else {
				fmt.Printf("   %s %s refreshed (%ds)\n",
					successStyle.Render("✔"), pluginNameStyle.Render(r.id), r.elapsed)
			}
		}
	}

	// ==========================================
	// Phase 6: Agent-Specific Plugins
	// ==========================================
	if runPlugins {
		printStepHeader(fmt.Sprintf("%d", currentStep), fmt.Sprintf("%d", totalSteps),
			"Updating agent-specific plugins...", "claude, codex, omp plugin updates")
		currentStep++

		pluginResults := runPhasePlugins(homeDir, enabledAgents, yesFlag, dryRunFlag)
		for _, r := range pluginResults {
			results = append(results, r)
			switch r.status {
			case statusFailed:
				fmt.Printf("   %s %s failed (exit %d in %ds)\n",
					errorStyle.Render("✖"), pluginNameStyle.Render(r.id), r.rc, r.elapsed)
			case statusUpToDate:
				fmt.Printf("   %s %s already up-to-date (%ds)\n",
					lipgloss.NewStyle().Foreground(infoColor).Render("✔"), pluginNameStyle.Render(r.id), r.elapsed)
			case statusDryRun:
				fmt.Printf("   %s %s simulated\n",
					lipgloss.NewStyle().Foreground(warnColor).Render("⚡"), pluginNameStyle.Render(r.id))
			default:
				fmt.Printf("   %s %s updated (%ds)\n",
					successStyle.Render("✔"), pluginNameStyle.Render(r.id), r.elapsed)
			}
		}
	}

	// ==========================================
	// Phase 7: Cache Pruning
	// ==========================================
	if runPlugins && !skipPruneFlag && enabledAgents["claude"] {
		printStepHeader(fmt.Sprintf("%d", currentStep), fmt.Sprintf("%d", totalSteps),
			"Pruning unused plugin cache...", "claude plugin prune -y (guarded by process checks)")
		currentStep++

		pruneRes, ok := runPhasePrune(homeDir, yesFlag, dryRunFlag, skipPruneFlag)
		if ok {
			results = append(results, pruneRes)
			if pruneRes.status == statusSkipped {
				fmt.Printf("   %s Cache pruning skipped\n", mutedColorTag("⊝"))
			} else if pruneRes.status == statusFailed {
				fmt.Printf("   %s Cache pruning failed (exit %d)\n", errorStyle.Render("✖"), pruneRes.rc)
			} else {
				fmt.Printf("   %s Prune completed (%ds)\n", successStyle.Render("✔"), pruneRes.elapsed)
			}
		}
	}

	// ==========================================
	// Phase 8: Results Summary & Reload Hints
	// ==========================================
	var updatedCount int
	var upToDateCount int
	var failedCount int
	var skippedCount int

	for _, r := range results {
		switch r.status {
		case statusUpdated:
			updatedCount++
		case statusUpToDate:
			upToDateCount++
		case statusFailed:
			failedCount++
		case statusSkipped:
			skippedCount++
		}
	}

	summaryHeader := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).Render("Results Summary")
	fmt.Printf("\n   %s\n", summaryHeader)

	tbl := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(secondaryColor)).
		Headers("#", "ITEM", "STATUS", "DURATION")

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
		case statusSkipped:
			statusText = lipgloss.NewStyle().Foreground(mutedColor).Render("⊝ Skipped")
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

	tableRendered := tbl.Render()
	for _, line := range strings.Split(tableRendered, "\n") {
		fmt.Printf("   %s\n", line)
	}

	totalItems := len(results)
	totalElapsed := int(time.Since(startTime).Seconds())
	var summaryText strings.Builder

	if dryRunFlag {
		summaryText.WriteString(fmt.Sprintf("%s Dry run completed for %d items %s\n",
			lipgloss.NewStyle().Foreground(warnColor).Render("⚡"),
			totalItems,
			mutedColorTag(fmt.Sprintf("(Total time: %ds)", totalElapsed)),
		))
	} else if failedCount == 0 {
		if updatedCount == 0 {
			summaryText.WriteString(fmt.Sprintf("%s All %d items are already up-to-date! %s\n",
				successStyle.Render("✔"),
				totalItems,
				mutedColorTag(fmt.Sprintf("(Total time: %ds)", totalElapsed)),
			))
		} else if upToDateCount == 0 {
			summaryText.WriteString(fmt.Sprintf("%s All %d items updated successfully! %s\n",
				successStyle.Render("✔"),
				totalItems,
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

	// Active Process Signals (Phase 8)
	var reloadHints []string
	if enabledAgents["omp"] && isProcessRunning("omp") {
		reloadHints = append(reloadHints, "• OMP session active: run /reload-plugins to activate changes.")
	}
	if enabledAgents["claude"] && isProcessRunning("claude") {
		reloadHints = append(reloadHints, "• Claude Code session active: restart session to activate changes.")
	}
	if enabledAgents["codex"] && isProcessRunning("codex") {
		reloadHints = append(reloadHints, "• Codex session active: restart application window to activate changes.")
	}

	if len(reloadHints) > 0 {
		summaryText.WriteString("\n" + lipgloss.NewStyle().Foreground(infoColor).Bold(true).Render("Active Processes:") + "\n")
		for _, hint := range reloadHints {
			summaryText.WriteString("  " + lipgloss.NewStyle().Foreground(warnColor).Render(hint) + "\n")
		}
	} else {
		summaryText.WriteString(mutedColorTag("Tip: Restart Claude or run /reload-plugins in omp to activate changes."))
	}

	fmt.Println()
	fmt.Println(summaryBox.Render(summaryText.String()))
	fmt.Println()
}
