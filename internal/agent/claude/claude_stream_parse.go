package claude

import (
	"bufio"
	"encoding/json"
	"strings"
)

func (d *Driver) parseStream(r interface{ Read([]byte) (int, error) }) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	toolName := map[int]string{}
	toolID := map[int]string{}
	toolInput := map[int]string{}
	maxCtx := 0

	for sc.Scan() {
		var line sjLine
		if json.Unmarshal(sc.Bytes(), &line) != nil {
			continue
		}
		switch line.Type {
		case "system":
			if line.Subtype == "compact_boundary" {
				d.emit(StreamEvent{Kind: "system", Text: "⤺ Context compacted — conversation summarized to shrink the context"})
			}
			if line.SessionID != "" {
				d.mu.Lock()
				d.session = line.SessionID
				d.mu.Unlock()
				d.emit(StreamEvent{Kind: "session", Session: line.SessionID, Model: line.Model})
			}
		case "stream_event":
			var ev sjEvent
			if json.Unmarshal(line.Event, &ev) != nil {
				continue
			}
			switch ev.Type {
			case "content_block_start":
				d.setCurText("")
				if ev.ContentBlock.Type == "tool_use" {
					toolName[ev.Index] = ev.ContentBlock.Name
					toolID[ev.Index] = ev.ContentBlock.ID
					toolInput[ev.Index] = ""
				}
			case "content_block_delta":
				if ev.Delta.Type == "text_delta" && ev.Delta.Text != "" {
					d.appendCurText(ev.Delta.Text)
					d.emit(StreamEvent{Kind: "text", Text: ev.Delta.Text})
				} else if ev.Delta.Type == "input_json_delta" {
					toolInput[ev.Index] += ev.Delta.PartialJSON
				}
			case "content_block_stop":
				if name, ok := toolName[ev.Index]; ok {
					d.emit(StreamEvent{Kind: "tool_use", Tool: name, ToolID: toolID[ev.Index],
						Input: json.RawMessage(orEmptyObj(toolInput[ev.Index]))})
					delete(toolName, ev.Index)
					delete(toolID, ev.Index)
					delete(toolInput, ev.Index)
				}
			}
		case "assistant":
			var am struct {
				Usage struct {
					InputTokens              int `json:"input_tokens"`
					CacheReadInputTokens     int `json:"cache_read_input_tokens"`
					CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
				} `json:"usage"`
			}
			if json.Unmarshal(line.Message, &am) == nil {
				used := am.Usage.InputTokens + am.Usage.CacheReadInputTokens + am.Usage.CacheCreationInputTokens
				if used > maxCtx {
					maxCtx = used
					d.emit(StreamEvent{Kind: "ctx", Ctx: pct(used)})
				}
			}
		case "user":
			for _, tr := range parseToolResults(line.Message) {
				d.emit(StreamEvent{Kind: "tool_result", ToolID: tr.id, Result: tr.text})
			}
		case "result":
			d.setCurText("")
			if line.SessionID != "" {
				d.mu.Lock()
				d.session = line.SessionID
				d.mu.Unlock()
			}
			if maxCtx >= autoCompactTokens {
				d.mu.Lock()
				d.wantCompact = true
				d.mu.Unlock()
			}
			u := line.Usage
			d.emit(StreamEvent{Kind: "done", Cost: line.TotalCost, Ctx: pct(maxCtx), Session: line.SessionID,
				CacheRead: u.CacheReadInputTokens, CacheWrite: u.CacheCreationInputTokens,
				InTok: u.InputTokens, OutTok: u.OutputTokens})
			if d.persistent {
				maxCtx = 0
				select {
				case d.turnEnd <- struct{}{}:
				default:
				}
			}
		case "rate_limit_event":
			var rl struct {
				Info struct {
					Status        string `json:"status"`
					ResetsAt      int64  `json:"resetsAt"`
					RateLimitType string `json:"rateLimitType"`
				} `json:"rate_limit_info"`
			}
			if json.Unmarshal(sc.Bytes(), &rl) == nil && rl.Info.RateLimitType != "" {
				d.emit(StreamEvent{Kind: "ratelimit", RateType: rl.Info.RateLimitType,
					ResetsAt: rl.Info.ResetsAt, RateStatus: rl.Info.Status})
			}
		}
	}
}

type sjLine struct {
	Type      string          `json:"type"`
	Subtype   string          `json:"subtype"`
	SessionID string          `json:"session_id"`
	Model     string          `json:"model"`
	TotalCost float64         `json:"total_cost_usd"`
	Event     json.RawMessage `json:"event"`
	Message   json.RawMessage `json:"message"`
	Usage     struct {
		InputTokens              int `json:"input_tokens"`
		CacheReadInputTokens     int `json:"cache_read_input_tokens"`
		CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
		OutputTokens             int `json:"output_tokens"`
	} `json:"usage"`
}

type sjEvent struct {
	Type  string `json:"type"`
	Index int    `json:"index"`
	Delta struct {
		Type        string `json:"type"`
		Text        string `json:"text"`
		PartialJSON string `json:"partial_json"`
	} `json:"delta"`
	ContentBlock struct {
		Type string `json:"type"`
		Name string `json:"name"`
		ID   string `json:"id"`
	} `json:"content_block"`
}

type toolResult struct{ id, text string }

func parseToolResults(msg json.RawMessage) []toolResult {
	var m struct {
		Content []struct {
			Type      string          `json:"type"`
			ToolUseID string          `json:"tool_use_id"`
			Content   json.RawMessage `json:"content"`
		} `json:"content"`
	}
	if json.Unmarshal(msg, &m) != nil {
		return nil
	}
	var out []toolResult
	for _, c := range m.Content {
		if c.Type != "tool_result" {
			continue
		}
		out = append(out, toolResult{id: c.ToolUseID, text: flattenContent(c.Content)})
	}
	return out
}

func flattenContent(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &parts) == nil {
		var b strings.Builder
		for _, p := range parts {
			b.WriteString(p.Text)
		}
		return b.String()
	}
	return ""
}

func orEmptyObj(s string) string {
	if strings.TrimSpace(s) == "" {
		return "{}"
	}
	return s
}
