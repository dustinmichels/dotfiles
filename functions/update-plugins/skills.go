package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// skillLock is skills.sh's global lock (~/.agents/.skill-lock.json): which repo each hub skill came from.
type skillLock struct {
	Skills map[string]lockEntry `json:"skills"`
}

type lockEntry struct {
	Source          string `json:"source"`
	SkillFolderHash string `json:"skillFolderHash"`
}

func readSkillLock(home string) skillLock {
	var l skillLock
	readJSON(skillsLockFile(home), &l)
	return l
}

// changedSkills lists skills whose content hash differs between two lock snapshots (or are new).
func changedSkills(before, after skillLock) map[string]bool {
	changed := map[string]bool{}
	for name, s := range after.Skills {
		if b, ok := before.Skills[name]; !ok || b.SkillFolderHash != s.SkillFolderHash {
			changed[name] = true
		}
	}
	return changed
}

// hubSkills maps hub skill name → path (dirs and symlinks to dirs, hidden entries skipped).
func hubSkills(hub string) map[string]string {
	skills := map[string]string{}
	entries, _ := os.ReadDir(hub)
	for _, e := range entries {
		p := filepath.Join(hub, e.Name())
		if fi, err := os.Stat(p); err == nil && fi.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			skills[e.Name()] = p
		}
	}
	return skills
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// skillRows updates skills.sh skills and reports every hub skill grouped by how it got there.
func (a *app) skillRows() []row {
	before := readSkillLock(a.home)
	var update row
	if len(before.Skills) > 0 {
		update = row{name: "skills.sh"}
		if npx := findNPX(a.home); npx == "" {
			update.raise(stFailed)
			update.detail = append(update.detail, "npx not found")
		} else {
			a.mutate(&update, slow, npx, "skills", "update", "-g", "-y")
		}
	}
	after := readSkillLock(a.home)
	changed := changedSkills(before, after)
	hub := hubSkills(hubDir(a.home))

	bySource := map[string][]string{}
	for name, s := range after.Skills {
		bySource[s.Source] = append(bySource[s.Source], name)
	}
	var rows []row
	for _, src := range sortedKeys(bySource) {
		names := bySource[src]
		sort.Strings(names)
		r := row{st: stCurrent, name: src, version: plural(len(names), "skill"), via: "skills.sh"}
		var updated, missing []string
		for _, n := range names {
			if changed[n] {
				updated = append(updated, n)
			}
			if _, ok := hub[n]; !ok {
				missing = append(missing, n)
			}
		}
		r.note = strings.Join(names, ", ")
		// The shared skills.sh command's outcome applies to every source row.
		r.raise(update.st)
		if update.st != stFailed && len(updated) > 0 {
			r.raise(stUpdated)
			r.note = "updated: " + strings.Join(updated, ", ")
		}
		if len(missing) > 0 {
			r.raise(stWarn)
			r.addNote("in lock but missing from hub: " + strings.Join(missing, ", "))
		}
		rows = append(rows, r)
	}
	// Show the shared command (planned or failed) once, under the first skills.sh row.
	if len(rows) > 0 {
		rows[0].detail = update.detail
	}

	for _, name := range sortedKeys(hub) {
		if _, ok := after.Skills[name]; ok {
			continue
		}
		if owner, ok := toolOwnedSkills[name]; ok {
			rows = append(rows, row{st: stInfo, name: name, version: "1 skill", via: owner, note: "updated in TOOLS"})
			continue
		}
		rows = append(rows, row{st: stWarn, name: name, version: "1 skill", via: "manual", note: "not tracked by skills.sh or a tool; update by hand"})
	}
	return rows
}

type spokeReport struct {
	agent, dir                            string
	linked, copies, local, healed, pruned []string
}

// reconcileSpoke makes every hub skill visible in an agent's skill dir: dangling symlinks are
// removed, missing skills are symlinked to the hub. Existing entries (copies, local skills) are left alone.
func reconcileSpoke(hub map[string]string, agent, dir string, dryRun bool) (spokeReport, error) {
	rep := spokeReport{agent: agent, dir: dir}
	if !dryRun {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return rep, err
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		return rep, err
	}

	present := map[string]bool{}
	for _, e := range entries {
		name := e.Name()
		p := filepath.Join(dir, name)
		fi, err := os.Lstat(p)
		if err != nil || strings.HasPrefix(name, ".") {
			continue
		}
		_, inHub := hub[name]
		if fi.Mode()&os.ModeSymlink != 0 {
			if _, err := os.Stat(p); err != nil {
				if !dryRun {
					if err := os.Remove(p); err != nil {
						return rep, err
					}
				}
				rep.pruned = append(rep.pruned, name)
				continue
			}
			present[name] = true
			if inHub {
				rep.linked = append(rep.linked, name)
			} else {
				rep.local = append(rep.local, name)
			}
			continue
		}
		present[name] = true
		if inHub {
			rep.copies = append(rep.copies, name)
		} else {
			rep.local = append(rep.local, name)
		}
	}

	for _, name := range sortedKeys(hub) {
		if present[name] {
			continue
		}
		if !dryRun {
			target := hub[name]
			if rel, err := filepath.Rel(dir, target); err == nil {
				target = rel
			}
			if err := os.Symlink(target, filepath.Join(dir, name)); err != nil {
				return rep, err
			}
		}
		rep.healed = append(rep.healed, name)
		rep.linked = append(rep.linked, name)
	}
	return rep, nil
}

// linkOmpSkills keeps ~/.omp/agent/skills as a single directory symlink to the hub.
// Returns whether the link was (or would be) created.
func linkOmpSkills(home string, dryRun bool) (bool, error) {
	hub, link := hubDir(home), ompSkillsDir(home)
	fi, err := os.Lstat(link)
	if err == nil {
		if fi.Mode()&os.ModeSymlink == 0 {
			return false, fmt.Errorf("%s is a real directory, not a link to the hub", link)
		}
		if _, err := os.Stat(link); err == nil {
			return false, nil
		}
	}
	if dryRun {
		return true, nil
	}
	if err == nil {
		if err := os.Remove(link); err != nil {
			return false, err
		}
	}
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		return false, err
	}
	target := hub
	if rel, err := filepath.Rel(filepath.Dir(link), hub); err == nil {
		target = rel
	}
	return true, os.Symlink(target, link)
}

// linkRows propagates the hub into each agent's skill dir and reports what each agent sees.
func (a *app) linkRows() []row {
	hubPath := hubDir(a.home)
	hub := hubSkills(hubPath)
	if len(hub) == 0 {
		return nil
	}
	changed := stUpdated
	if a.dryRun {
		changed = stPlanned
	}

	var rows []row
	for _, s := range spokeDirs(a.home) {
		if !a.agents[s.agent] {
			continue
		}
		r := row{st: stCurrent, name: s.agent, via: tilde(a.home, s.dir)}
		rep, err := reconcileSpoke(hub, s.agent, s.dir, a.dryRun)
		if err != nil {
			r.raise(stFailed)
			r.detail = append(r.detail, err.Error())
		}
		r.version = fmt.Sprintf("%d/%d", len(rep.linked)+len(rep.copies), len(hub))
		r.addNote(fmt.Sprintf("%d linked", len(rep.linked)))
		if len(rep.copies) > 0 {
			r.addNote(fmt.Sprintf("%d copied (%s)", len(rep.copies), strings.Join(rep.copies, ", ")))
		}
		if len(rep.local) > 0 {
			r.addNote("local only: " + strings.Join(rep.local, ", "))
		}
		if len(rep.healed) > 0 {
			r.raise(changed)
			r.addNote("linked " + strings.Join(rep.healed, ", "))
		}
		if len(rep.pruned) > 0 {
			r.raise(changed)
			r.addNote("removed dead links " + strings.Join(rep.pruned, ", "))
		}
		var stale []string
		for _, name := range rep.copies {
			if dirHash(filepath.Join(s.dir, name)) != dirHash(hub[name]) {
				stale = append(stale, name)
			}
		}
		if len(stale) > 0 {
			r.raise(stWarn)
			r.addNote("copy differs from hub: " + strings.Join(stale, ", "))
		}
		rows = append(rows, r)
	}

	if a.agents["omp"] {
		r := row{st: stCurrent, name: "omp", version: fmt.Sprintf("%d/%d", len(hub), len(hub)), via: tilde(a.home, ompSkillsDir(a.home)), note: "directory link → hub"}
		created, err := linkOmpSkills(a.home, a.dryRun)
		switch {
		case err != nil:
			r.raise(stWarn)
			r.version = ""
			r.note = err.Error()
		case created:
			r.raise(changed)
			r.addNote("relinked")
		}
		rows = append(rows, r)
	}
	return rows
}
