package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/pomelohq/pomelo/internal/paths"
)

type Tool struct {
	Name           string
	Description    string
	Schema         map[string]any
	ReadOnly       bool
	Destructive    bool
	MaxResultChars int
	Run            func(args map[string]any) (string, error)
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

func Serve(in io.Reader, out io.Writer, name, version string, tools []Tool) error {
	byName := make(map[string]Tool, len(tools))
	for _, t := range tools {
		byName[t.Name] = t
	}
	w := bufio.NewWriter(out)
	send := func(id json.RawMessage, result any, rpcErr *rpcError) {
		if id == nil {
			return
		}
		msg := map[string]any{"jsonrpc": "2.0", "id": id}
		if rpcErr != nil {
			msg["error"] = rpcErr
		} else {
			msg["result"] = result
		}
		b, _ := json.Marshal(msg)
		w.Write(b)
		w.WriteByte('\n')
		w.Flush()
	}

	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 1024*1024), 16*1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var req rpcRequest
		if json.Unmarshal(line, &req) != nil {
			continue
		}
		switch req.Method {
		case "initialize":
			send(req.ID, initResult(req.Params, name, version), nil)
		case "notifications/initialized", "notifications/cancelled":
		case "ping":
			send(req.ID, map[string]any{}, nil)
		case "tools/list":
			send(req.ID, map[string]any{"tools": toolList(tools)}, nil)
		case "tools/call":
			res, rerr := callTool(byName, req.Params)
			send(req.ID, res, rerr)
		default:
			send(req.ID, nil, &rpcError{Code: -32601, Message: "method not found: " + req.Method})
		}
	}
	return sc.Err()
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func initResult(params json.RawMessage, name, version string) map[string]any {
	proto := "2025-06-18"
	var p struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	if json.Unmarshal(params, &p) == nil && p.ProtocolVersion != "" {
		proto = p.ProtocolVersion
	}
	return map[string]any{
		"protocolVersion": proto,
		"capabilities":    map[string]any{"tools": map[string]any{}},
		"serverInfo":      map[string]any{"name": name, "version": version},
	}
}

func toolList(tools []Tool) []map[string]any {
	out := make([]map[string]any, 0, len(tools))
	for _, t := range tools {
		schema := t.Schema
		if schema == nil {
			schema = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		entry := map[string]any{
			"name": t.Name, "description": t.Description, "inputSchema": schema,
		}
		entry["annotations"] = map[string]any{
			"readOnlyHint":    t.ReadOnly,
			"destructiveHint": t.Destructive && !t.ReadOnly,
			"idempotentHint":  t.ReadOnly,
		}
		out = append(out, entry)
	}
	return out
}

func callTool(byName map[string]Tool, params json.RawMessage) (any, *rpcError) {
	var p struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if json.Unmarshal(params, &p) != nil {
		return nil, &rpcError{Code: -32602, Message: "invalid params"}
	}
	t, ok := byName[p.Name]
	if !ok {
		return textResult(fmt.Sprintf("unknown tool: %s", p.Name), true), nil
	}
	text, err := t.Run(p.Arguments)
	if err != nil {
		return textResult(err.Error(), true), nil
	}
	if t.MaxResultChars > 0 && len(text) > t.MaxResultChars {
		text = capResult(t.Name, text, t.MaxResultChars)
	}
	return textResult(text, false), nil
}

func capResult(tool, text string, max int) string {
	dir := filepath.Join(paths.StateDir(), "mcp-out")
	_ = os.MkdirAll(dir, 0o755)
	h := fnv.New32a()
	_, _ = h.Write([]byte(text))
	path := filepath.Join(dir, fmt.Sprintf("%s-%08x.txt", tool, h.Sum32()))
	_ = os.WriteFile(path, []byte(text), 0o644)

	head := text[:max]
	if i := strings.LastIndexByte(head, '\n'); i > max/2 {
		head = head[:i]
	}
	lines := strings.Count(text, "\n") + 1
	return fmt.Sprintf("%s\n\n… [truncated: %d of %d chars shown · %d lines total]\n"+
		"Full output saved to %s — read it (Read tool / `cat`) if you need the rest.",
		head, len(head), len(text), lines, path)
}

func textResult(text string, isErr bool) map[string]any {
	return map[string]any{
		"content": []map[string]any{{"type": "text", "text": text}},
		"isError": isErr,
	}
}
