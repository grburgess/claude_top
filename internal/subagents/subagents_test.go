package subagents

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestCount(t *testing.T) {
	sessionDir := t.TempDir()
	dir := filepath.Join(sessionDir, "subagents")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("agent-1.jsonl", `{"type":"assistant"}`+"\n")
	write("agent-1.meta.json", `{"agentType":"Explore","description":"d","toolUseId":"toolu_1","spawnDepth":1,"model":"haiku"}`)
	write("agent-2.jsonl", `{"type":"assistant"}`+"\n")
	write("agent-2.meta.json", `{"agentType":"Plan"}`)
	// agent-2 is stale
	old := time.Now().Add(-10 * time.Minute)
	if err := os.Chtimes(filepath.Join(dir, "agent-2.jsonl"), old, old); err != nil {
		t.Fatal(err)
	}
	// meta without jsonl must not count
	write("agent-3.meta.json", `{"agentType":"Ghost"}`)
	// jsonl without meta counts, name falls back to stem
	write("agent-4.jsonl", "x\n")

	live, total, names := Count(sessionDir, 120*time.Second)
	if live != 2 {
		t.Errorf("live = %d, want 2", live)
	}
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}
	want := []string{"Explore", "Plan", "agent-4"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("names = %v, want %v", names, want)
	}
}

func TestCountMissingDir(t *testing.T) {
	live, total, names := Count(t.TempDir(), time.Minute)
	if live != 0 || total != 0 || names != nil {
		t.Errorf("got %d %d %v, want zeros", live, total, names)
	}
}
