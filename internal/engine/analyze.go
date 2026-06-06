// Package engine runs the blast-radius analysis: resolve a target via a language
// server, then map its direct references and transitive callers.
package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/buddyh/blast-radius/internal/lsp"
)

// Options configure an analysis run.
type Options struct {
	Target string // symbol name, or file:line[:col]
	Dir    string // workspace root (default: cwd)
	Depth  int    // transitive caller depth (default 3)
	Lang   string // optional server name/langID override
}

// Consumer is one place affected by changing the target.
type Consumer struct {
	File string `json:"file"`
	Line int    `json:"line"`
	Via  string `json:"via,omitempty"` // for ripple: the caller it reaches through
}

// Report is the full blast-radius result.
type Report struct {
	Target    string     `json:"target"`
	Def       string     `json:"def"`       // file:line of the definition
	Kind      string     `json:"kind"`      // symbol kind
	Lang      string     `json:"lang"`      // language server used
	Breaking  []Consumer `json:"breaking"`  // direct references
	Ripple    []Consumer `json:"ripple"`    // transitive callers (depth >= 2)
	Tests     []Consumer `json:"tests"`     // references inside test files
	Ambiguous []string   `json:"ambiguous,omitempty"`
	Depth     int        `json:"depth"`
}

var fileLineRe = regexp.MustCompile(`^(.+):(\d+)(?::(\d+))?$`)

func isTest(path string) bool {
	p := strings.ToLower(path)
	for _, m := range []string{"_test.", ".test.", ".spec.", "/test/", "/tests/", "/__tests__/"} {
		if strings.Contains(p, m) {
			return true
		}
	}
	return false
}

// Analyze resolves the target and maps its blast radius.
func Analyze(ctx context.Context, o Options) (*Report, error) {
	dir := o.Dir
	if dir == "" {
		dir, _ = os.Getwd()
	}
	dir, _ = filepath.Abs(dir)
	if o.Depth <= 0 {
		o.Depth = 3
	}

	srv, defFile, ok := pickServer(dir, o)
	if !ok {
		return nil, fmt.Errorf("no installed language server for this target; run `br doctor`")
	}
	if !srv.Installed() {
		return nil, fmt.Errorf("%s language server not installed — %s", srv.Lang, srv.Install)
	}

	cmd := append([]string{srv.Path()}, srv.Cmd[1:]...) // launch by resolved path (may be off-PATH)
	cl, err := lsp.Start(cmd)
	if err != nil {
		return nil, err
	}
	defer cl.Close()
	if err := cl.Initialize(ctx, dir); err != nil {
		return nil, err
	}

	rep := &Report{
		Target: o.Target, Lang: srv.Lang, Depth: o.Depth,
		Breaking: []Consumer{}, Ripple: []Consumer{}, Tests: []Consumer{},
	}

	var uri string
	var pos lsp.Position
	if defFile != nil { // target was file:line[:col]
		uri = lsp.PathToURI(defFile.path)
		pos = lsp.Position{Line: defFile.line, Character: defFile.col}
		_ = cl.DidOpen(defFile.path, srv.LangID)
		rep.Def = fmt.Sprintf("%s:%d", rel(dir, defFile.path), defFile.line+1)
		rep.Kind = "symbol"
	} else { // target is a symbol name
		syms, err := cl.WorkspaceSymbol(ctx, o.Target)
		if err != nil {
			return nil, err
		}
		matches := exactSymbols(syms, o.Target)
		if len(matches) == 0 {
			return nil, fmt.Errorf("symbol %q not found in %s", o.Target, dir)
		}
		def := matches[0]
		uri = def.Location.URI
		pos = def.Location.Range.Start
		_ = cl.DidOpen(lsp.URIToPath(uri), srv.LangID)
		rep.Def = fmt.Sprintf("%s:%d", rel(dir, lsp.URIToPath(uri)), pos.Line+1)
		rep.Kind = lsp.SymbolKind[def.Kind]
		for _, m := range matches[1:] {
			rep.Ambiguous = append(rep.Ambiguous, fmt.Sprintf("%s:%d", rel(dir, lsp.URIToPath(m.Location.URI)), m.Location.Range.Start.Line+1))
		}
	}

	// Direct references = breaking consumers.
	refs, _ := cl.References(ctx, uri, pos, false)
	seen := map[string]bool{}
	for _, r := range refs {
		p := lsp.URIToPath(r.URI)
		c := Consumer{File: rel(dir, p), Line: r.Range.Start.Line + 1}
		key := c.File + ":" + strconv.Itoa(c.Line)
		if seen[key] {
			continue
		}
		seen[key] = true
		if isTest(p) {
			rep.Tests = append(rep.Tests, c)
		} else {
			rep.Breaking = append(rep.Breaking, c)
		}
	}

	// Transitive callers (depth >= 2) = ripple.
	items, _ := cl.PrepareCallHierarchy(ctx, uri, pos)
	visited := map[string]bool{}
	for _, it := range items {
		level1, _ := cl.IncomingCalls(ctx, it)
		for _, c1 := range level1 {
			visited[itemKey(c1.From)] = true // direct caller; already breaking
			walkRipple(ctx, cl, c1.From, 2, o.Depth, visited, seen, dir, rep)
		}
	}

	sortConsumers(rep.Breaking)
	sortConsumers(rep.Ripple)
	sortConsumers(rep.Tests)
	return rep, nil
}

func walkRipple(ctx context.Context, cl *lsp.Client, item lsp.CallHierarchyItem, depth, max int, visited, seen map[string]bool, dir string, rep *Report) {
	if depth > max {
		return
	}
	calls, _ := cl.IncomingCalls(ctx, item)
	for _, c := range calls {
		k := itemKey(c.From)
		if visited[k] {
			continue
		}
		visited[k] = true
		p := lsp.URIToPath(c.From.URI)
		cons := Consumer{File: rel(dir, p), Line: c.From.SelectionRange.Start.Line + 1, Via: item.Name}
		key := cons.File + ":" + strconv.Itoa(cons.Line)
		if !seen[key] {
			seen[key] = true
			if isTest(p) {
				rep.Tests = append(rep.Tests, cons)
			} else {
				rep.Ripple = append(rep.Ripple, cons)
			}
		}
		walkRipple(ctx, cl, c.From, depth+1, max, visited, seen, dir, rep)
	}
}

func itemKey(i lsp.CallHierarchyItem) string {
	return i.URI + ":" + strconv.Itoa(i.SelectionRange.Start.Line)
}

func exactSymbols(syms []lsp.SymbolInformation, name string) []lsp.SymbolInformation {
	var exact, suffix []lsp.SymbolInformation
	for _, s := range syms {
		switch {
		case s.Name == name:
			exact = append(exact, s)
		case strings.HasSuffix(s.Name, "."+name):
			suffix = append(suffix, s)
		}
	}
	out := append(exact, suffix...)
	// definitions (functions/types) before fields/vars
	sort.SliceStable(out, func(a, b int) bool { return kindRank(out[a].Kind) < kindRank(out[b].Kind) })
	return out
}

func kindRank(k int) int {
	switch k {
	case 12, 6, 9: // function, method, constructor
		return 0
	case 5, 11, 23, 26: // class, interface, struct, type
		return 1
	default:
		return 2
	}
}

func sortConsumers(cs []Consumer) {
	sort.SliceStable(cs, func(a, b int) bool {
		if cs[a].File != cs[b].File {
			return cs[a].File < cs[b].File
		}
		return cs[a].Line < cs[b].Line
	})
}

func rel(dir, p string) string {
	if r, err := filepath.Rel(dir, p); err == nil && !strings.HasPrefix(r, "..") {
		return r
	}
	return p
}

type fileLine struct {
	path string
	line int // zero-based
	col  int // zero-based
}

// pickServer decides which language server to use and, if the target is
// file:line[:col], returns the resolved location.
func pickServer(dir string, o Options) (lsp.Server, *fileLine, bool) {
	if m := fileLineRe.FindStringSubmatch(o.Target); m != nil {
		path := m[1]
		if !filepath.IsAbs(path) {
			path = filepath.Join(dir, path)
		}
		if fi, err := os.Stat(path); err == nil && !fi.IsDir() {
			line, _ := strconv.Atoi(m[2])
			col := 1
			if m[3] != "" {
				col, _ = strconv.Atoi(m[3])
			}
			if s, ok := lsp.ForFile(path); ok {
				return s, &fileLine{path: path, line: line - 1, col: col - 1}, true
			}
		}
	}
	if o.Lang != "" {
		for _, s := range lsp.Servers {
			if strings.EqualFold(s.Lang, o.Lang) || strings.EqualFold(s.LangID, o.Lang) {
				return s, nil, true
			}
		}
	}
	if s, ok := dominantServer(dir); ok {
		return s, nil, true
	}
	return lsp.Server{}, nil, false
}

// dominantServer picks the installed server matching the most source files in dir.
func dominantServer(dir string) (lsp.Server, bool) {
	counts := map[string]int{}
	skip := map[string]bool{"node_modules": true, "vendor": true, ".git": true, "dist": true, "build": true, "target": true}
	filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && skip[d.Name()] {
			return filepath.SkipDir
		}
		if !d.IsDir() {
			if s, ok := lsp.ForFile(p); ok {
				counts[s.Lang]++
			}
		}
		return nil
	})
	best, bestN := lsp.Server{}, 0
	found := false
	for _, s := range lsp.Servers {
		if s.Installed() && counts[s.Lang] > bestN {
			best, bestN, found = s, counts[s.Lang], true
		}
	}
	return best, found
}
