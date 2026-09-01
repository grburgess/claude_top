// Package subagents counts subagent transcripts for a session.
package subagents

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type meta struct {
	AgentType   string `json:"agentType"`
	Description string `json:"description"`
	ToolUseID   string `json:"toolUseId"`
	SpawnDepth  int    `json:"spawnDepth"`
	Model       string `json:"model"`
}

// Count inspects sessionDir/subagents/. Total is the number of
// agent-*.jsonl files; a subagent is live when its .jsonl mtime is within
// liveWindow of now. Names come from the sibling .meta.json agentType
// (falling back to the file stem), sorted for determinism.
func Count(sessionDir string, liveWindow time.Duration) (live, total int, names []string) {
	dir := filepath.Join(sessionDir, "subagents")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, 0, nil
	}
	now := time.Now()
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasPrefix(name, "agent-") || !strings.HasSuffix(name, ".jsonl") {
			continue
		}
		total++
		if info, err := e.Info(); err == nil && now.Sub(info.ModTime()) <= liveWindow {
			live++
		}
		stem := strings.TrimSuffix(name, ".jsonl")
		display := stem
		if data, err := os.ReadFile(filepath.Join(dir, stem+".meta.json")); err == nil {
			var m meta
			if json.Unmarshal(data, &m) == nil && m.AgentType != "" {
				display = m.AgentType
			}
		}
		names = append(names, display)
	}
	sort.Strings(names)
	return live, total, names
}
