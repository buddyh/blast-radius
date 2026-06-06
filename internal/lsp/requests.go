package lsp

import (
	"context"
	"encoding/json"
	"os"
)

// Position is a zero-based line/character offset.
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// Range is a span between two positions.
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Location is a range within a document.
type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

// SymbolInformation is a workspace/symbol result.
type SymbolInformation struct {
	Name          string   `json:"name"`
	Kind          int      `json:"kind"`
	Location      Location `json:"location"`
	ContainerName string   `json:"containerName"`
}

// CallHierarchyItem identifies a callable for call-hierarchy queries.
type CallHierarchyItem struct {
	Name           string `json:"name"`
	Kind           int    `json:"kind"`
	URI            string `json:"uri"`
	Range          Range  `json:"range"`
	SelectionRange Range  `json:"selectionRange"`
	Detail         string `json:"detail,omitempty"`
}

// CallHierarchyIncomingCall is a caller of a CallHierarchyItem.
type CallHierarchyIncomingCall struct {
	From       CallHierarchyItem `json:"from"`
	FromRanges []Range           `json:"fromRanges"`
}

// SymbolKind names for the kinds we care about (LSP spec values).
var SymbolKind = map[int]string{
	5: "class", 6: "method", 8: "field", 9: "constructor", 11: "interface",
	12: "function", 13: "variable", 14: "constant", 23: "struct", 26: "type",
}

func textDoc(uri string) map[string]any { return map[string]any{"uri": uri} }

// DidOpen tells the server about a file's contents (required before queries).
func (c *Client) DidOpen(path, langID string) error {
	text, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return c.Notify("textDocument/didOpen", map[string]any{
		"textDocument": map[string]any{
			"uri": PathToURI(path), "languageId": langID, "version": 1, "text": string(text),
		},
	})
}

// WorkspaceSymbol resolves a symbol name across the workspace.
func (c *Client) WorkspaceSymbol(ctx context.Context, query string) ([]SymbolInformation, error) {
	raw, err := c.Call(ctx, "workspace/symbol", map[string]any{"query": query})
	if err != nil {
		return nil, err
	}
	var out []SymbolInformation
	_ = json.Unmarshal(raw, &out)
	return out, nil
}

// References returns every reference to the symbol at a position.
func (c *Client) References(ctx context.Context, uri string, pos Position, includeDecl bool) ([]Location, error) {
	raw, err := c.Call(ctx, "textDocument/references", map[string]any{
		"textDocument": textDoc(uri),
		"position":     pos,
		"context":      map[string]any{"includeDeclaration": includeDecl},
	})
	if err != nil {
		return nil, err
	}
	var out []Location
	_ = json.Unmarshal(raw, &out)
	return out, nil
}

// PrepareCallHierarchy resolves the callable at a position for hierarchy queries.
func (c *Client) PrepareCallHierarchy(ctx context.Context, uri string, pos Position) ([]CallHierarchyItem, error) {
	raw, err := c.Call(ctx, "textDocument/prepareCallHierarchy", map[string]any{
		"textDocument": textDoc(uri), "position": pos,
	})
	if err != nil {
		return nil, err
	}
	var out []CallHierarchyItem
	_ = json.Unmarshal(raw, &out)
	return out, nil
}

// IncomingCalls returns the direct callers of a call-hierarchy item.
func (c *Client) IncomingCalls(ctx context.Context, item CallHierarchyItem) ([]CallHierarchyIncomingCall, error) {
	raw, err := c.Call(ctx, "callHierarchy/incomingCalls", map[string]any{"item": item})
	if err != nil {
		return nil, err
	}
	var out []CallHierarchyIncomingCall
	_ = json.Unmarshal(raw, &out)
	return out, nil
}
