// claude_top monitors Claude Code sessions. UI to come; for now:
//
//	claude_top --inspect <session-id>   reduce a transcript, print Stats JSON
//	claude_top                          count sessions via the registry
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/burgessj/claude_top/internal/registry"
	"github.com/burgessj/claude_top/internal/transcript"
)

func main() {
	args := os.Args[1:]
	if len(args) >= 1 && args[0] == "--inspect" {
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: claude_top --inspect <session-id>")
			os.Exit(2)
		}
		if err := inspect(args[1]); err != nil {
			fmt.Fprintln(os.Stderr, "claude_top:", err)
			os.Exit(1)
		}
		return
	}
	reg := registry.New("", "")
	if err := reg.Refresh(); err != nil {
		fmt.Fprintln(os.Stderr, "claude_top:", err)
		os.Exit(1)
	}
	fmt.Printf("TUI coming: %d sessions found\n", len(reg.List(registry.ModeAll)))
}

func inspect(sessionID string) error {
	path, err := findTranscript(registry.DefaultProjectsDir(), sessionID)
	if err != nil {
		return err
	}
	var st transcript.Stats
	var r transcript.Reader
	if err := r.Tail(path, &st); err != nil {
		return err
	}
	out, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(out))
	return nil
}

func findTranscript(projectsDir, sessionID string) (string, error) {
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return "", fmt.Errorf("read projects dir %s: %w", projectsDir, err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		p := filepath.Join(projectsDir, e.Name(), sessionID+".jsonl")
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("transcript for session %s not found under %s", sessionID, projectsDir)
}
