// Package transcript is a streaming reducer over Claude Code session
// transcripts (JSONL).
package transcript

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"strings"
	"time"
)

const tokensPerTurnCap = 100

// Stats is the accumulated view of one session transcript.
type Stats struct {
	Model          string
	Effort         string
	Version        string
	GitBranch      string
	Title          string
	Mode           string
	PermissionMode string

	ContextTokens  int64 // last cache_read + cache_creation + input
	ContextLimit   int64
	TotalIn        int64
	TotalOut       int64
	ThinkingTokens int64
	CostUSD        float64
	Turns          int

	Skills     []string
	MCPServers []string
	Workflows  int

	LastToolCall      string
	LastPromptSnippet string
	TokensPerTurn     []int64 // out tokens per assistant turn, ring capped

	FirstTs time.Time
	LastTs  time.Time
}

// Reader tails a transcript file, advancing Offset monotonically past
// complete lines only.
type Reader struct {
	Offset int64
}

type usage struct {
	InputTokens              int64 `json:"input_tokens"`
	OutputTokens             int64 `json:"output_tokens"`
	CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
	OutputTokensDetails      *struct {
		ThinkingTokens int64 `json:"thinking_tokens"`
	} `json:"output_tokens_details"`
}

type contentItem struct {
	Type  string          `json:"type"`
	Name  string          `json:"name"`
	Text  string          `json:"text"`
	Input json.RawMessage `json:"input"`
}

type line struct {
	Type    string `json:"type"`
	Message *struct {
		Model   string          `json:"model"`
		Usage   *usage          `json:"usage"`
		Content json.RawMessage `json:"content"`
	} `json:"message"`
	Timestamp            string `json:"timestamp"`
	GitBranch            string `json:"gitBranch"`
	Version              string `json:"version"`
	Effort               string `json:"effort"`
	AiTitle              string `json:"aiTitle"`
	Mode                 string `json:"mode"`
	PermissionMode       string `json:"permissionMode"`
	AttributionSkill     string `json:"attributionSkill"`
	AttributionMcpServer string `json:"attributionMcpServer"`
}

// Tail reads from r.Offset to the last complete line of path, updating st.
// Malformed or unknown lines are skipped silently. An incomplete final line
// (no trailing newline) is not consumed and Offset is not advanced past it.
func (r *Reader) Tail(path string, st *Stats) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Seek(r.Offset, io.SeekStart); err != nil {
		return err
	}
	br := bufio.NewReaderSize(f, 256*1024)
	for {
		data, err := br.ReadBytes('\n')
		if err != nil {
			// Incomplete final line: do not advance offset past it.
			return nil
		}
		r.Offset += int64(len(data))
		reduceLine(data, st)
	}
}

func reduceLine(data []byte, st *Stats) {
	defer func() { _ = recover() }() // never panic on hostile input
	var ln line
	if err := json.Unmarshal(data, &ln); err != nil {
		return
	}
	if ts, err := time.Parse(time.RFC3339, ln.Timestamp); err == nil {
		if st.FirstTs.IsZero() {
			st.FirstTs = ts
		}
		if ts.After(st.LastTs) {
			st.LastTs = ts
		}
	}
	if ln.AttributionSkill != "" {
		st.Skills = addUnique(st.Skills, ln.AttributionSkill)
	}
	if ln.AttributionMcpServer != "" {
		st.MCPServers = addUnique(st.MCPServers, ln.AttributionMcpServer)
	}
	switch ln.Type {
	case "assistant":
		reduceAssistant(&ln, st)
	case "user":
		reduceUser(&ln, st)
	case "ai-title":
		st.Title = ln.AiTitle
	case "mode":
		st.Mode = ln.Mode
	case "permission-mode":
		st.PermissionMode = ln.PermissionMode
	default:
		// unknown types skipped silently
	}
}

func reduceAssistant(ln *line, st *Stats) {
	st.Turns++
	if ln.GitBranch != "" {
		st.GitBranch = ln.GitBranch
	}
	if ln.Version != "" {
		st.Version = ln.Version
	}
	if ln.Effort != "" {
		st.Effort = ln.Effort
	}
	if ln.Message == nil {
		return
	}
	if ln.Message.Model != "" {
		st.Model = ln.Message.Model
		st.ContextLimit = contextLimitFor(ln.Message.Model)
	}
	if u := ln.Message.Usage; u != nil {
		st.ContextTokens = u.CacheReadInputTokens + u.CacheCreationInputTokens + u.InputTokens
		st.TotalIn += u.InputTokens
		st.TotalOut += u.OutputTokens
		if u.OutputTokensDetails != nil {
			st.ThinkingTokens += u.OutputTokensDetails.ThinkingTokens
		}
		if p, ok := priceFor(st.Model); ok {
			st.CostUSD += float64(u.InputTokens)*p.in/1e6 +
				float64(u.OutputTokens)*p.out/1e6 +
				float64(u.CacheCreationInputTokens)*p.cacheWrite/1e6 +
				float64(u.CacheReadInputTokens)*p.cacheRead/1e6
		}
		st.TokensPerTurn = append(st.TokensPerTurn, u.OutputTokens)
		if len(st.TokensPerTurn) > tokensPerTurnCap {
			st.TokensPerTurn = st.TokensPerTurn[len(st.TokensPerTurn)-tokensPerTurnCap:]
		}
	}
	var items []contentItem
	if err := json.Unmarshal(ln.Message.Content, &items); err != nil {
		return
	}
	for _, it := range items {
		if it.Type != "tool_use" {
			continue
		}
		st.LastToolCall = it.Name
		switch {
		case it.Name == "Workflow":
			st.Workflows++
		case it.Name == "Skill":
			var in struct {
				Skill string `json:"skill"`
			}
			if json.Unmarshal(it.Input, &in) == nil && in.Skill != "" {
				st.Skills = addUnique(st.Skills, in.Skill)
			}
		case strings.HasPrefix(it.Name, "mcp__"):
			rest := strings.TrimPrefix(it.Name, "mcp__")
			if i := strings.Index(rest, "__"); i > 0 {
				st.MCPServers = addUnique(st.MCPServers, rest[:i])
			}
		}
	}
}

func reduceUser(ln *line, st *Stats) {
	if ln.Message == nil {
		return
	}
	var prompt string
	if err := json.Unmarshal(ln.Message.Content, &prompt); err != nil {
		return // arrays are tool_results etc., not real prompts
	}
	st.LastPromptSnippet = snippet(prompt, 80)
}

func snippet(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	r := []rune(s)
	if len(r) > max {
		return string(r[:max])
	}
	return s
}

func addUnique(list []string, v string) []string {
	for _, x := range list {
		if x == v {
			return list
		}
	}
	return append(list, v)
}
