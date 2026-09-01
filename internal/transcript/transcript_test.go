package transcript

import (
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func writeTranscript(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "s.jsonl")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func tailAll(t *testing.T, content string) *Stats {
	t.Helper()
	p := writeTranscript(t, content)
	var st Stats
	var r Reader
	if err := r.Tail(p, &st); err != nil {
		t.Fatal(err)
	}
	return &st
}

func TestUsageAccumulationAndCost(t *testing.T) {
	jsonl := `{"type":"assistant","message":{"model":"claude-opus-4","usage":{"input_tokens":100,"output_tokens":200,"cache_creation_input_tokens":1000,"cache_read_input_tokens":5000,"output_tokens_details":{"thinking_tokens":40}},"content":[{"type":"text","text":"hi"}]},"timestamp":"2026-01-01T10:00:00Z","gitBranch":"main","version":"2.1.3","effort":"high","uuid":"u1"}
{"type":"assistant","message":{"model":"claude-opus-4","usage":{"input_tokens":50,"output_tokens":100,"cache_creation_input_tokens":0,"cache_read_input_tokens":6000},"content":[{"type":"tool_use","id":"toolu_1","name":"Bash","input":{"command":"ls"}}]},"timestamp":"2026-01-01T10:05:00Z","uuid":"u2"}
`
	st := tailAll(t, jsonl)
	if st.Turns != 2 {
		t.Errorf("Turns = %d, want 2", st.Turns)
	}
	if st.Model != "claude-opus-4" {
		t.Errorf("Model = %q", st.Model)
	}
	if st.Effort != "high" || st.Version != "2.1.3" || st.GitBranch != "main" {
		t.Errorf("meta = %q %q %q", st.Effort, st.Version, st.GitBranch)
	}
	if st.TotalIn != 150 || st.TotalOut != 300 {
		t.Errorf("TotalIn/Out = %d/%d, want 150/300", st.TotalIn, st.TotalOut)
	}
	if st.ThinkingTokens != 40 {
		t.Errorf("ThinkingTokens = %d, want 40", st.ThinkingTokens)
	}
	if st.ContextTokens != 6050 {
		t.Errorf("ContextTokens = %d, want 6050", st.ContextTokens)
	}
	if st.ContextLimit != 200_000 {
		t.Errorf("ContextLimit = %d, want 200000", st.ContextLimit)
	}
	wantCost := 0.04275 + 0.01725
	if math.Abs(st.CostUSD-wantCost) > 1e-9 {
		t.Errorf("CostUSD = %v, want %v", st.CostUSD, wantCost)
	}
	if !reflect.DeepEqual(st.TokensPerTurn, []int64{200, 100}) {
		t.Errorf("TokensPerTurn = %v", st.TokensPerTurn)
	}
	if st.LastToolCall != "Bash" {
		t.Errorf("LastToolCall = %q", st.LastToolCall)
	}
	if st.FirstTs.IsZero() || !st.LastTs.After(st.FirstTs) {
		t.Errorf("FirstTs/LastTs = %v/%v", st.FirstTs, st.LastTs)
	}
}

func TestUnknownModelZeroCost(t *testing.T) {
	jsonl := `{"type":"assistant","message":{"model":"mystery-model","usage":{"input_tokens":1000,"output_tokens":1000}},"timestamp":"2026-01-01T10:00:00Z"}
`
	st := tailAll(t, jsonl)
	if st.CostUSD != 0 {
		t.Errorf("CostUSD = %v, want 0", st.CostUSD)
	}
}

func TestContextLimit1M(t *testing.T) {
	for _, model := range []string{"claude-fable-5", "claude-sonnet-4[1m]"} {
		jsonl := `{"type":"assistant","message":{"model":"` + model + `","usage":{"input_tokens":1}},"timestamp":"2026-01-01T10:00:00Z"}
`
		st := tailAll(t, jsonl)
		if st.ContextLimit != 1_000_000 {
			t.Errorf("%s: ContextLimit = %d, want 1000000", model, st.ContextLimit)
		}
	}
}

func TestTitleModeCapture(t *testing.T) {
	jsonl := `{"type":"ai-title","aiTitle":"Fix the parser"}
{"type":"mode","mode":"plan"}
{"type":"permission-mode","permissionMode":"acceptEdits"}
{"type":"user","message":{"content":"please fix   the\nparser now"}}
`
	st := tailAll(t, jsonl)
	if st.Title != "Fix the parser" {
		t.Errorf("Title = %q", st.Title)
	}
	if st.Mode != "plan" {
		t.Errorf("Mode = %q", st.Mode)
	}
	if st.PermissionMode != "acceptEdits" {
		t.Errorf("PermissionMode = %q", st.PermissionMode)
	}
	if st.LastPromptSnippet != "please fix the parser now" {
		t.Errorf("LastPromptSnippet = %q", st.LastPromptSnippet)
	}
}

func TestToolResultNotPrompt(t *testing.T) {
	jsonl := `{"type":"user","message":{"content":"real prompt"}}
{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"toolu_1"}]}}
`
	st := tailAll(t, jsonl)
	if st.LastPromptSnippet != "real prompt" {
		t.Errorf("LastPromptSnippet = %q, want real prompt", st.LastPromptSnippet)
	}
}

func TestSkillsAndMCPDedup(t *testing.T) {
	jsonl := `{"type":"assistant","attributionSkill":"deep-research","message":{"model":"claude-haiku-3","usage":{"input_tokens":1},"content":[]},"timestamp":"2026-01-01T10:00:00Z"}
{"type":"assistant","attributionSkill":"deep-research","message":{"model":"claude-haiku-3","content":[{"type":"tool_use","id":"toolu_2","name":"Skill","input":{"skill":"boston"}}]},"timestamp":"2026-01-01T10:01:00Z"}
{"type":"assistant","attributionMcpServer":"qgis","message":{"model":"claude-haiku-3","content":[{"type":"tool_use","id":"toolu_3","name":"mcp__atlassian__search","input":{}},{"type":"tool_use","id":"toolu_4","name":"Workflow","input":{}}]},"timestamp":"2026-01-01T10:02:00Z"}
{"type":"assistant","attributionMcpServer":"qgis","message":{"model":"claude-haiku-3","content":[]},"timestamp":"2026-01-01T10:03:00Z"}
`
	st := tailAll(t, jsonl)
	if !reflect.DeepEqual(st.Skills, []string{"deep-research", "boston"}) {
		t.Errorf("Skills = %v", st.Skills)
	}
	if !reflect.DeepEqual(st.MCPServers, []string{"qgis", "atlassian"}) {
		t.Errorf("MCPServers = %v", st.MCPServers)
	}
	if st.Workflows != 1 {
		t.Errorf("Workflows = %d, want 1", st.Workflows)
	}
	if st.LastToolCall != "Workflow" {
		t.Errorf("LastToolCall = %q", st.LastToolCall)
	}
}

func TestUnknownTypeAndMalformedSkipped(t *testing.T) {
	jsonl := `{"type":"attachment","stuff":true}
{"type":"totally-new-thing","x":1}
this line is not json at all
{"broken json
{"type":"assistant","message":{"model":"claude-sonnet-4","usage":{"input_tokens":10,"output_tokens":20}},"timestamp":"2026-01-01T10:00:00Z"}
`
	st := tailAll(t, jsonl)
	if st.Turns != 1 {
		t.Errorf("Turns = %d, want 1", st.Turns)
	}
	if st.Model != "claude-sonnet-4" {
		t.Errorf("Model = %q", st.Model)
	}
}

func TestIncompleteFinalLineOffset(t *testing.T) {
	p := filepath.Join(t.TempDir(), "s.jsonl")
	complete := `{"type":"assistant","message":{"model":"claude-haiku-3","usage":{"input_tokens":1,"output_tokens":2}},"timestamp":"2026-01-01T10:00:00Z"}` + "\n"
	partial := `{"type":"assistant","message":{"model":"claude-haiku-3","usage":{"inpu`
	if err := os.WriteFile(p, []byte(complete+partial), 0o644); err != nil {
		t.Fatal(err)
	}
	var st Stats
	var r Reader
	if err := r.Tail(p, &st); err != nil {
		t.Fatal(err)
	}
	if r.Offset != int64(len(complete)) {
		t.Errorf("Offset = %d, want %d (must not pass incomplete line)", r.Offset, len(complete))
	}
	if st.Turns != 1 {
		t.Errorf("Turns = %d, want 1", st.Turns)
	}

	// Complete the partial line and append another; Tail again.
	rest := `t_tokens":5,"output_tokens":7}},"timestamp":"2026-01-01T10:01:00Z"}` + "\n"
	f, err := os.OpenFile(p, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(rest); err != nil {
		t.Fatal(err)
	}
	f.Close()

	before := r.Offset
	if err := r.Tail(p, &st); err != nil {
		t.Fatal(err)
	}
	if r.Offset <= before {
		t.Errorf("Offset not monotonic: %d -> %d", before, r.Offset)
	}
	if r.Offset != int64(len(complete)+len(partial)+len(rest)) {
		t.Errorf("Offset = %d, want %d", r.Offset, len(complete)+len(partial)+len(rest))
	}
	if st.Turns != 2 {
		t.Errorf("Turns = %d, want 2 (partial line completed)", st.Turns)
	}
	if st.TotalIn != 6 || st.TotalOut != 9 {
		t.Errorf("TotalIn/Out = %d/%d, want 6/9", st.TotalIn, st.TotalOut)
	}
}

func TestOffsetMonotonicNoNewData(t *testing.T) {
	p := writeTranscript(t, `{"type":"mode","mode":"plan"}`+"\n")
	var st Stats
	var r Reader
	if err := r.Tail(p, &st); err != nil {
		t.Fatal(err)
	}
	before := r.Offset
	if err := r.Tail(p, &st); err != nil {
		t.Fatal(err)
	}
	if r.Offset != before {
		t.Errorf("Offset moved with no new data: %d -> %d", before, r.Offset)
	}
}

func TestTokensPerTurnRingCap(t *testing.T) {
	var content string
	for i := 0; i < 150; i++ {
		content += `{"type":"assistant","message":{"model":"claude-haiku-3","usage":{"input_tokens":1,"output_tokens":1}},"timestamp":"2026-01-01T10:00:00Z"}` + "\n"
	}
	st := tailAll(t, content)
	if len(st.TokensPerTurn) != 100 {
		t.Errorf("len(TokensPerTurn) = %d, want 100", len(st.TokensPerTurn))
	}
	if st.Turns != 150 {
		t.Errorf("Turns = %d, want 150", st.Turns)
	}
}
