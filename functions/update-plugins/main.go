package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/x/term"
)

type app struct {
	home                 string
	dryRun, yes, verbose bool
	agents               map[string]bool
	tty                  bool
	width                int
	progressShown        bool
	counts               map[status]int
}

var sectionNames = []string{"tools", "skills", "plugins"}

const usage = `update-plugins — inventory and update AI tools, skills, and plugins

Shows every agent, external tool, skill, and plugin: its version, how it was
installed, and where it propagates. Then updates each through its own
installer so changes reach every agent.

USAGE
  update-plugins [flags]

FLAGS
  -d, --dry-run         Show the inventory and the commands that would run; change nothing
  -y, --yes             Confirm prompts: passes -y to claude plugin update, runs claude plugin prune
  -v, --verbose         Stream command output
      --only <list>     Sections to run: tools,skills,plugins (default: all)
      --agents <list>   Agents to include: claude,codex,omp,pi,gemini (default: all)
  -h, --help            Show this help

EXAMPLES
  update-plugins -d                 # what's installed and how
  update-plugins                    # update everything
  update-plugins --only skills      # skills.sh + hub links only
  update-plugins --agents claude    # only Claude's plugins and skill links
`

func parseList(arg string, valid []string) (map[string]bool, error) {
	allowed := map[string]bool{}
	for _, v := range valid {
		allowed[v] = true
	}
	out := map[string]bool{}
	for _, p := range strings.Split(arg, ",") {
		name := strings.ToLower(strings.TrimSpace(p))
		if name == "" {
			continue
		}
		if !allowed[name] {
			return nil, fmt.Errorf("unknown value %q (valid: %s)", name, strings.Join(valid, ", "))
		}
		out[name] = true
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("empty list (valid: %s)", strings.Join(valid, ", "))
	}
	return out, nil
}

func fail(msg string) {
	fmt.Fprintf(os.Stderr, "%s %s\n", errStyle.Render("✖"), msg)
	os.Exit(2)
}

func main() {
	home, err := os.UserHomeDir()
	if err != nil {
		fail("cannot resolve home directory: " + err.Error())
	}
	a := &app{home: home, counts: map[status]int{}, width: 100}
	only, _ := parseList(strings.Join(sectionNames, ","), sectionNames)
	a.agents, _ = parseList(strings.Join(agentNames, ","), agentNames)

	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		arg := args[i]
		value := func(flag string) string {
			if v, ok := strings.CutPrefix(arg, flag+"="); ok {
				return v
			}
			if i+1 >= len(args) {
				fail(flag + " requires a comma-separated list")
			}
			i++
			return args[i]
		}
		switch {
		case arg == "-d" || arg == "--dry-run":
			a.dryRun = true
		case arg == "-y" || arg == "--yes":
			a.yes = true
		case arg == "-v" || arg == "--verbose":
			a.verbose = true
		case arg == "--only" || strings.HasPrefix(arg, "--only="):
			if only, err = parseList(value("--only"), sectionNames); err != nil {
				fail("--only: " + err.Error())
			}
		case arg == "--agents" || strings.HasPrefix(arg, "--agents="):
			if a.agents, err = parseList(value("--agents"), agentNames); err != nil {
				fail("--agents: " + err.Error())
			}
		case arg == "-h" || arg == "--help":
			fmt.Print(usage)
			return
		default:
			fmt.Fprint(os.Stderr, usage)
			fail("unknown flag: " + arg)
		}
	}

	if fd := os.Stdout.Fd(); term.IsTerminal(fd) {
		a.tty = true
		if w, _, err := term.GetSize(fd); err == nil && w > 0 {
			a.width = w
		}
	}

	start := time.Now()
	header := titleStyle.Render("AI tools") + dimStyle.Render("  inventory & update")
	if a.dryRun {
		header += "  " + badgeStyle.Render("DRY RUN")
	}
	fmt.Println(header)

	a.progress("reading agents")
	a.section("AGENTS", "the hosts; each updates itself", a.agentRows())
	if only["tools"] {
		a.section("TOOLS", "external tools that install skills/hooks into agents", a.toolRows())
	}
	if only["skills"] {
		a.section("SKILLS", tilde(home, hubDir(home))+" · shared hub, by source", a.skillRows())
		a.section("SKILL LINKS", "hub → each agent's skill dir", a.linkRows())
	}
	if only["plugins"] {
		if a.agents["claude"] {
			a.section("CLAUDE PLUGINS", "", a.claudePluginRows())
		}
		if a.agents["codex"] {
			a.section("CODEX PLUGINS", "", a.codexPluginRows())
		}
		if a.agents["omp"] {
			a.section("OMP PLUGINS", "", a.ompPluginRows())
		}
	}
	a.clearProgress()
	a.summary(time.Since(start))
	if a.counts[stFailed] > 0 {
		os.Exit(1)
	}
}

func (a *app) summary(elapsed time.Duration) {
	var parts []string
	add := func(st status, label string) {
		if n := a.counts[st]; n > 0 {
			parts = append(parts, st.glyph()+" "+fmt.Sprintf("%d %s", n, label))
		}
	}
	add(stUpdated, "updated")
	add(stPlanned, "would update")
	add(stCurrent, "current")
	add(stWarn, "need attention")
	add(stFailed, "failed")
	fmt.Printf("\n%s  %s\n", strings.Join(parts, "   "), dimStyle.Render(fmt.Sprintf("%.0fs", elapsed.Seconds())))

	if a.dryRun {
		fmt.Println(dimStyle.Render("dry run: nothing changed"))
		return
	}
	if a.counts[stUpdated] == 0 {
		return
	}
	var hints []string
	if a.agents["claude"] && isProcessRunning("claude") {
		hints = append(hints, "restart Claude Code sessions")
	}
	if a.agents["codex"] && isProcessRunning("codex") {
		hints = append(hints, "restart Codex")
	}
	if a.agents["omp"] && isProcessRunning("omp") {
		hints = append(hints, "run /reload-plugins in omp")
	}
	if len(hints) > 0 {
		fmt.Println(warnStyle.Render("running sessions keep old versions: " + strings.Join(hints, "; ")))
	}
}
