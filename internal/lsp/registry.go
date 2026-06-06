// Package lsp drives language servers (LSP) to resolve symbols, references, and
// call hierarchies for blast-radius analysis.
package lsp

import (
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

// Path returns the resolved binary path, or "" if the server isn't on PATH.
func (s Server) Path() string {
	p, _ := exec.LookPath(s.Cmd[0])
	return p
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
