// Package lsp drives language servers (LSP) to resolve symbols, references, and
// call hierarchies for blast-radius analysis.
package lsp

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Server describes a language server: how to launch it over stdio and how to
// install it if it's missing.
type Server struct {
	Lang    string   // display name, e.g. "Go"
	Exts    []string // source extensions it handles, incl. leading dot
	Cmd     []string // launch command + args (stdio JSON-RPC)
	LangID  string   // LSP languageId for textDocument/didOpen
	Install string   // one-line install hint
}

// Servers is the registry of supported language servers, in priority order.
var Servers = []Server{
	{"Go", []string{".go"}, []string{"gopls"}, "go",
		"go install golang.org/x/tools/gopls@latest"},
	{"TypeScript/JavaScript", []string{".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs"},
		[]string{"typescript-language-server", "--stdio"}, "typescript",
		"npm install -g typescript-language-server typescript"},
	{"Python", []string{".py", ".pyi"}, []string{"pyright-langserver", "--stdio"}, "python",
		"npm install -g pyright"},
	{"Rust", []string{".rs"}, []string{"rust-analyzer"}, "rust",
		"rustup component add rust-analyzer"},
	{"C/C++", []string{".c", ".cc", ".cpp", ".cxx", ".h", ".hpp", ".hh"}, []string{"clangd"}, "cpp",
		"brew install llvm   # provides clangd"},
}

// Path returns the resolved binary path. It checks PATH first, then common
// install locations (~/go/bin, ~/.cargo/bin, …) so servers installed by
// toolchains that aren't on PATH are still found. Returns "" if not found.
func (s Server) Path() string {
	if p, err := exec.LookPath(s.Cmd[0]); err == nil {
		return p
	}
	for _, dir := range fallbackBinDirs() {
		cand := filepath.Join(dir, s.Cmd[0])
		if fi, err := os.Stat(cand); err == nil && !fi.IsDir() && fi.Mode()&0o111 != 0 {
			return cand
		}
	}
	return ""
}

// fallbackBinDirs are common per-toolchain bin directories that are often not
// on PATH in a non-login shell.
func fallbackBinDirs() []string {
	home, _ := os.UserHomeDir()
	dirs := []string{}
	if gp := os.Getenv("GOPATH"); gp != "" {
		dirs = append(dirs, filepath.Join(gp, "bin"))
	}
	dirs = append(dirs,
		filepath.Join(home, "go", "bin"),
		filepath.Join(home, ".cargo", "bin"),
		filepath.Join(home, ".local", "bin"),
		"/opt/homebrew/bin", "/usr/local/bin",
	)
	return dirs
}

// Installed reports whether the server binary is available.
func (s Server) Installed() bool { return s.Path() != "" }

// ForExt returns the server that handles a file extension (incl. leading dot).
func ForExt(ext string) (Server, bool) {
	ext = strings.ToLower(ext)
	for _, s := range Servers {
		for _, e := range s.Exts {
			if e == ext {
				return s, true
			}
		}
	}
	return Server{}, false
}

// ForFile returns the server that handles a file path by its extension.
func ForFile(path string) (Server, bool) {
	return ForExt(filepath.Ext(path))
}
