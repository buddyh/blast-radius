package engine

import (
	"encoding/json"
	"fmt"
	"io"
)

// Render writes the report as JSON (for tooling) or a table (for humans).
func (r *Report) Render(w io.Writer, format string) {
	if format == "json" {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		_ = enc.Encode(r)
		return
	}

	fmt.Fprintf(w, "\nblast radius · %s\n", r.Target)
	if r.Def != "" {
		fmt.Fprintf(w, "  def     %s  (%s · %s)\n", r.Def, r.Kind, r.Lang)
	} else {
		fmt.Fprintf(w, "  kind    %s\n", r.Kind)
	}
	fmt.Fprintf(w, "  impact  %d breaking · %d ripple · %d test\n",
		len(r.Breaking), len(r.Ripple), len(r.Tests))
	if r.Note != "" {
		fmt.Fprintf(w, "  note    %s\n", r.Note)
	}
	if len(r.Ambiguous) > 0 {
		fmt.Fprintf(w, "  note    %d other symbols share this name — pass file:line to disambiguate\n", len(r.Ambiguous))
	}

	section(w, "BREAKING  (direct references — update with the change)", r.Breaking)
	section(w, "RIPPLE  (transitive callers — review for behavior change)", r.Ripple)
	section(w, "TESTS  (re-run these)", r.Tests)

	level, strategy := risk(r)
	fmt.Fprintf(w, "\n  risk    %s — %s\n\n", level, strategy)
}

func section(w io.Writer, title string, cs []Consumer) {
	if len(cs) == 0 {
		return
	}
	fmt.Fprintf(w, "\n  %s\n", title)
	for _, c := range cs {
		if c.Via != "" {
			fmt.Fprintf(w, "    %s:%d  ← via %s\n", c.File, c.Line, c.Via)
		} else {
			fmt.Fprintf(w, "    %s:%d\n", c.File, c.Line)
		}
	}
}

func risk(r *Report) (level, strategy string) {
	n := len(r.Breaking) + len(r.Ripple)
	switch {
	case n == 0:
		return "NONE", "no consumers found — may be dead code, or resolution missed it"
	case len(r.Breaking) <= 3:
		return "LOW", "change in place and update consumers in the same commit"
	case len(r.Breaking) <= 10:
		return "MEDIUM", "add the new version alongside the old, migrate consumers, then remove"
	default:
		return "HIGH", "strangler pattern — new symbol + deprecation, phased migration"
	}
}

// FilesTouched returns the unique set of files across all impact categories.
func (r *Report) FilesTouched() []string {
	seen := map[string]bool{}
	var out []string
	for _, set := range [][]Consumer{r.Breaking, r.Ripple, r.Tests} {
		for _, c := range set {
			if !seen[c.File] {
				seen[c.File] = true
				out = append(out, c.File)
			}
		}
	}
	return out
}
