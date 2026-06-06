package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Client is a minimal LSP client speaking JSON-RPC 2.0 over a server's stdio.
type Client struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	writeMu sync.Mutex

	mu      sync.Mutex
	nextID  int
	pending map[int]chan rpcResponse
	closed  chan struct{}
}

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type rpcResponse struct {
	ID     *int            `json:"id"`
	Method string          `json:"method"`
	Result json.RawMessage `json:"result"`
	Error  *rpcError       `json:"error"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Start launches the language server and begins reading its output.
func Start(command []string) (*Client, error) {
	cmd := exec.Command(command[0], command[1:]...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start %s: %w", command[0], err)
	}
	c := &Client{cmd: cmd, stdin: stdin, pending: map[int]chan rpcResponse{}, closed: make(chan struct{})}
	go c.read(stdout)
	return c, nil
}

func (c *Client) writeMsg(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if _, err := fmt.Fprintf(c.stdin, "Content-Length: %d\r\n\r\n", len(b)); err != nil {
		return err
	}
	_, err = c.stdin.Write(b)
	return err
}

func (c *Client) read(r io.Reader) {
	defer close(c.closed)
	br := bufio.NewReader(r)
	for {
		length := 0
		for {
			line, err := br.ReadString('\n')
			if err != nil {
				return
			}
			line = strings.TrimRight(line, "\r\n")
			if line == "" {
				break
			}
			if v, ok := strings.CutPrefix(line, "Content-Length:"); ok {
				length, _ = strconv.Atoi(strings.TrimSpace(v))
			}
		}
		if length == 0 {
			continue
		}
		body := make([]byte, length)
		if _, err := io.ReadFull(br, body); err != nil {
			return
		}
		var msg rpcResponse
		if json.Unmarshal(body, &msg) != nil {
			continue
		}
		switch {
		case msg.Method != "" && msg.ID != nil:
			// server→client request: reply with a null result so it never blocks.
			_ = c.writeMsg(map[string]any{"jsonrpc": "2.0", "id": *msg.ID, "result": nil})
		case msg.Method != "":
			// notification: ignore.
		case msg.ID != nil:
			c.mu.Lock()
			ch := c.pending[*msg.ID]
			c.mu.Unlock()
			if ch != nil {
				ch <- msg
			}
		}
	}
}

// Call sends a request and waits for its response.
func (c *Client) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	c.mu.Lock()
	c.nextID++
	id := c.nextID
	ch := make(chan rpcResponse, 1)
	c.pending[id] = ch
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
	}()

	if err := c.writeMsg(rpcRequest{JSONRPC: "2.0", ID: id, Method: method, Params: params}); err != nil {
		return nil, err
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.closed:
		return nil, fmt.Errorf("lsp: server exited")
	case resp := <-ch:
		if resp.Error != nil {
			return nil, fmt.Errorf("lsp %s: %s", method, resp.Error.Message)
		}
		return resp.Result, nil
	}
}

// Notify sends a notification (no response expected).
func (c *Client) Notify(method string, params any) error {
	return c.writeMsg(rpcRequest{JSONRPC: "2.0", Method: method, Params: params})
}

// Initialize performs the LSP initialize handshake rooted at dir.
func (c *Client) Initialize(ctx context.Context, dir string) error {
	params := map[string]any{
		"processId": nil,
		"rootUri":   PathToURI(dir),
		"capabilities": map[string]any{
			"textDocument": map[string]any{
				"references":    map[string]any{},
				"definition":    map[string]any{},
				"callHierarchy": map[string]any{"dynamicRegistration": false},
			},
			"workspace": map[string]any{"symbol": map[string]any{}},
		},
	}
	if _, err := c.Call(ctx, "initialize", params); err != nil {
		return err
	}
	return c.Notify("initialized", map[string]any{})
}

// Close shuts the server down and waits for it to exit.
func (c *Client) Close() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, _ = c.Call(ctx, "shutdown", nil)
	_ = c.Notify("exit", nil)
	_ = c.stdin.Close()
	if c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
		_, _ = c.cmd.Process.Wait()
	}
}

// PathToURI converts an absolute file path to a file:// URI.
func PathToURI(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	return "file://" + (&url.URL{Path: abs}).EscapedPath()
}

// URIToPath converts a file:// URI back to a filesystem path.
func URIToPath(uri string) string {
	u, err := url.Parse(uri)
	if err != nil {
		return strings.TrimPrefix(uri, "file://")
	}
	return u.Path
}
