package transcript

import (
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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

func TestUsageDedupPerMessageID(t *testing.T) {
	// Claude Code emits one assistant line per content block, each
	// repeating the same cumulative message.usage. Three lines sharing one
	// message.id with identical usage must count exactly once.
	ln := `{"type":"assistant","message":{"id":"msg_01","model":"claude-opus-4","usage":{"input_tokens":100,"output_tokens":200,"cache_creation_input_tokens":1000,"cache_read_input_tokens":5000},"content":[{"type":"text","text":"a"}]},"timestamp":"2026-01-01T10:00:00Z"}` + "\n"
	st := tailAll(t, ln+ln+ln)
	if st.Turns != 1 {
		t.Errorf("Turns = %d, want 1 (one message.id)", st.Turns)
	}
	// Measured inflation before the fix: 3x on every counter.
	if st.TotalIn != 100 || st.TotalOut != 200 {
		t.Errorf("TotalIn/Out = %d/%d, want 100/200 (was inflated 3x)", st.TotalIn, st.TotalOut)
	}
	wantCost := 100*15.0/1e6 + 200*75.0/1e6 + 1000*18.75/1e6 + 5000*1.5/1e6
	if math.Abs(st.CostUSD-wantCost) > 1e-9 {
		t.Errorf("CostUSD = %v, want %v (was inflated 3x)", st.CostUSD, wantCost)
	}
	if !reflect.DeepEqual(st.TokensPerTurn, []int64{200}) {
		t.Errorf("TokensPerTurn = %v, want [200]", st.TokensPerTurn)
	}
}

func TestUsageNoMessageIDEachLineOwnMessage(t *testing.T) {
	dup := `{"type":"assistant","message":{"id":"msg_02","model":"claude-haiku-3","usage":{"input_tokens":10,"output_tokens":20}},"timestamp":"2026-01-01T10:00:00Z"}` + "\n"
	noID := `{"type":"assistant","message":{"model":"claude-haiku-3","usage":{"input_tokens":1,"output_tokens":2}},"timestamp":"2026-01-01T10:01:00Z"}` + "\n"
	st := tailAll(t, dup+dup+noID+noID)
	if st.Turns != 3 {
		t.Errorf("Turns = %d, want 3 (msg_02 once + 2 id-less lines)", st.Turns)
	}
	if st.TotalIn != 12 || st.TotalOut != 24 {
		t.Errorf("TotalIn/Out = %d/%d, want 12/24", st.TotalIn, st.TotalOut)
	}
}

func TestContextLimitPromotion1M(t *testing.T) {
	// Plain "claude-opus-5" gives no 1M hint; observed context past 95% of
	// 200k must promote the limit to the next tier.
	jsonl := `{"type":"assistant","message":{"id":"m1","model":"claude-opus-5","usage":{"input_tokens":100,"output_tokens":10,"cache_creation_input_tokens":3626,"cache_read_input_tokens":400000}},"timestamp":"2026-01-01T10:00:00Z"}
{"type":"assistant","message":{"id":"m2","model":"claude-opus-5","usage":{"input_tokens":1,"output_tokens":1}},"timestamp":"2026-01-01T10:01:00Z"}
`
	st := tailAll(t, jsonl)
	if st.ContextTokens != 1 {
		t.Errorf("ContextTokens = %d, want 1 (last message)", st.ContextTokens)
	}
	if st.ContextLimit != 1_000_000 {
		t.Errorf("ContextLimit = %d, want 1000000 (promoted, not demoted by later lines)", st.ContextLimit)
	}
}

func TestContextLimitPromotionSingleLine(t *testing.T) {
	jsonl := `{"type":"assistant","message":{"id":"m1","model":"claude-opus-5","usage":{"input_tokens":100,"output_tokens":10,"cache_creation_input_tokens":3626,"cache_read_input_tokens":400000}},"timestamp":"2026-01-01T10:00:00Z"}
`
	st := tailAll(t, jsonl)
	if st.ContextTokens != 403726 {
		t.Errorf("ContextTokens = %d, want 403726", st.ContextTokens)
	}
	if st.ContextLimit != 1_000_000 {
		t.Errorf("ContextLimit = %d, want 1000000", st.ContextLimit)
	}
}

func TestNegativeTokensClamped(t *testing.T) {
	jsonl := `{"type":"assistant","message":{"id":"m1","model":"claude-opus-4","usage":{"input_tokens":100,"output_tokens":-50,"cache_creation_input_tokens":-10,"cache_read_input_tokens":-1}},"timestamp":"2026-01-01T10:00:00Z"}
`
	st := tailAll(t, jsonl)
	if st.TotalOut != 0 {
		t.Errorf("TotalOut = %d, want 0 (negative clamped)", st.TotalOut)
	}
	if st.TotalIn != 100 {
		t.Errorf("TotalIn = %d, want 100", st.TotalIn)
	}
	if st.ContextTokens != 100 {
		t.Errorf("ContextTokens = %d, want 100 (negatives clamped)", st.ContextTokens)
	}
	if !reflect.DeepEqual(st.TokensPerTurn, []int64{0}) {
		t.Errorf("TokensPerTurn = %v, want [0]", st.TokensPerTurn)
	}
	wantCost := 100 * 15.0 / 1e6 // only positive input priced
	if math.Abs(st.CostUSD-wantCost) > 1e-9 {
		t.Errorf("CostUSD = %v, want %v", st.CostUSD, wantCost)
	}
}

func TestLastAssistantText(t *testing.T) {
	jsonl := `{"type":"assistant","message":{"model":"claude-opus-4","content":[{"type":"text","text":"first\nreply"}]},"timestamp":"2026-01-01T10:00:00Z"}
{"type":"assistant","message":{"model":"claude-opus-4","content":[{"type":"tool_use","id":"t1","name":"Bash","input":{}}]},"timestamp":"2026-01-01T10:01:00Z"}
{"type":"assistant","message":{"model":"claude-opus-4","content":[{"type":"text","text":"  "},{"type":"text","text":"Done.  Both\nprojects\tare in the graph."}]},"timestamp":"2026-01-01T10:02:00Z"}
`
	st := tailAll(t, jsonl)
	if st.LastAssistantText != "Done. Both projects are in the graph." {
		t.Errorf("LastAssistantText = %q", st.LastAssistantText)
	}
}

func TestLastAssistantTextCapped300(t *testing.T) {
	long := strings.Repeat("word ", 100)
	jsonl := `{"type":"assistant","message":{"content":[{"type":"text","text":"` + long + `"}]},"timestamp":"2026-01-01T10:00:00Z"}
`
	st := tailAll(t, jsonl)
	if n := len([]rune(st.LastAssistantText)); n != 300 {
		t.Errorf("LastAssistantText len = %d, want 300", n)
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
