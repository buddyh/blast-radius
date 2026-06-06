package engine

import (
	"fmt"
	"os/exec"
	"sort"
	"strings"
)

// CoChangeEntry is a file that historically changes together with the target.
type CoChangeEntry struct {
	File    string  `json:"file"`
	Commits int     `json:"commits"`
	Pct     float64 `json:"pct"` // share of the target's commits that also touched this file
}

// CoChange finds files that most often changed in the same commits as file,
// over the last maxCommits commits that touched it — temporal coupling that
// static analysis (the language server) cannot see. Paths are relative to the
// git root. Returns the top limit by frequency.
func CoChange(dir, file string, limit, maxCommits int) []CoChangeEntry {
	root := gitRoot(dir)
	if root == "" {
		return nil
	}
	out, err := git(root, "log", "--no-merges", "--format=%H", fmt.Sprintf("-n%d", maxCommits), "--", file)
	if err != nil {
		return nil
	}
	hashes := nonEmpty(strings.Split(string(out), "\n"))
	if len(hashes) < 2 {
		return nil // not enough history to be meaningful
	}

	args := append([]string{"show", "--name-only", "--format=format:%x00"}, hashes...)
	out, err = git(root, args...)
	if err != nil {
		return nil
	}
	targetRel := rel(root, file)
	counts := map[string]int{}
	for _, chunk := range strings.Split(string(out), "\x00") {
		for _, f := range nonEmpty(strings.Split(chunk, "\n")) {
			if f == targetRel {
				continue
			}
			counts[f]++
		}
	}

	entries := make([]CoChangeEntry, 0, len(counts))
	for f, n := range counts {
		entries = append(entries, CoChangeEntry{File: f, Commits: n, Pct: float64(n) / float64(len(hashes))})
	}
	sort.SliceStable(entries, func(a, b int) bool {
		if entries[a].Commits != entries[b].Commits {
			return entries[a].Commits > entries[b].Commits
		}
		return entries[a].File < entries[b].File
	})
	if limit > 0 && len(entries) > limit {
		entries = entries[:limit]
	}
	return entries
}

func gitRoot(dir string) string {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func git(root string, args ...string) ([]byte, error) {
	return exec.Command("git", append([]string{"-C", root}, args...)...).Output()
}

func nonEmpty(lines []string) []string {
	var out []string
	for _, l := range lines {
		if s := strings.TrimSpace(l); s != "" {
			out = append(out, s)
		}
	}
	return out
}
