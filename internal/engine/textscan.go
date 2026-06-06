package engine

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// scanRes holds one file's matches split by test/non-test.
type scanRes struct{ b, t []Consumer }

// textScan greps every file under dir (whose extension is in exts, or all when
// exts is nil) for re, returning consumers split into non-test and test.
func textScan(dir string, re *regexp.Regexp, exts map[string]bool) (breaking, tests []Consumer) {
	skip := map[string]bool{
		"node_modules": true, "vendor": true, ".git": true, "dist": true,
		"build": true, "target": true, ".next": true, ".venv": true,
	}
	var files []string
	filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if skip[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if exts == nil || exts[strings.ToLower(filepath.Ext(p))] {
			files = append(files, p)
		}
		return nil
	})

	results := make([]scanRes, len(files))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 24)
	for i, f := range files {
		wg.Add(1)
		go func(i int, f string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[i] = scanFile(f, re, dir)
		}(i, f)
	}
	wg.Wait()

	for _, r := range results {
		breaking = append(breaking, r.b...)
		tests = append(tests, r.t...)
	}
	sortConsumers(breaking)
	sortConsumers(tests)
	return breaking, tests
}

func scanFile(path string, re *regexp.Regexp, dir string) scanRes {
	f, err := os.Open(path)
	if err != nil {
		return scanRes{}
	}
	defer f.Close()
	rd := bufio.NewReader(f)
	if head, _ := rd.Peek(512); bytes.IndexByte(head, 0) >= 0 {
		return scanRes{} // binary file — skip
	}
	sc := bufio.NewScanner(rd)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	var res scanRes
	test := isTest(path)
	rp := rel(dir, path)
	ln := 0
	for sc.Scan() {
		ln++
		if re.MatchString(sc.Text()) {
			c := Consumer{File: rp, Line: ln}
			if test {
				res.t = append(res.t, c)
			} else {
				res.b = append(res.b, c)
			}
		}
	}
	return res
}

// extension sets for text-based resolution.
var codeExts = set(".go", ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs", ".py", ".rb",
	".java", ".kt", ".swift", ".rs", ".php", ".cs", ".scala", ".sql", ".graphql")

var configExts = set(".env", ".yaml", ".yml", ".json", ".toml", ".ini", ".conf",
	".properties", ".tf", ".sh", ".go", ".ts", ".js", ".py", ".rb", ".java", ".rs")

var dataExts = set(".sql", ".prisma", ".rb", ".py", ".go", ".ts", ".js", ".java", ".kt", ".graphql")

func set(items ...string) map[string]bool {
	m := make(map[string]bool, len(items))
	for _, i := range items {
		m[i] = true
	}
	return m
}
