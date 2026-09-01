// Package signalfile reads Claude Code tab-status signal files.
package signalfile

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Signal is one parsed <session-id>.json signal file.
type Signal struct {
	SessionID string
	Type      string // "running" | "idle" | "attention"
	Message   string
	Project   string
	Cwd       string
	TTY       string
	Pid       int
	Ts        time.Time
	Activity  string
}

// rawSignal mirrors the on-disk format: all values are strings.
type rawSignal struct {
	SessionID string `json:"session_id"`
	Type      string `json:"type"`
	Message   string `json:"message"`
	Project   string `json:"project"`
	Cwd       string `json:"cwd"`
	TTY       string `json:"tty"`
	Pid       string `json:"pid"`
	Ts        string `json:"ts"`
	Activity  string `json:"activity"`
}

// DefaultDir returns the signal directory, honoring CLAUDE_TOP_SIGNAL_DIR.
func DefaultDir() string {
	if d := os.Getenv("CLAUDE_TOP_SIGNAL_DIR"); d != "" {
		return d
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".cache/claude-tab-status"
	}
	return filepath.Join(home, ".cache", "claude-tab-status")
}

// Parse reads and decodes a single signal file. A missing session_id is an
// error; unparsable pid/ts fields yield zero values without error.
func Parse(path string) (Signal, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Signal{}, err
	}
	var raw rawSignal
	if err := json.Unmarshal(data, &raw); err != nil {
		return Signal{}, err
	}
	if raw.SessionID == "" {
		return Signal{}, errors.New("signalfile: missing session_id")
	}
	s := Signal{
		SessionID: raw.SessionID,
		Type:      raw.Type,
		Message:   raw.Message,
		Project:   raw.Project,
		Cwd:       raw.Cwd,
		TTY:       raw.TTY,
		Activity:  raw.Activity,
	}
	if pid, err := strconv.Atoi(raw.Pid); err == nil {
		s.Pid = pid
	}
	if secs, err := strconv.ParseInt(raw.Ts, 10, 64); err == nil {
		s.Ts = time.Unix(secs, 0)
	}
	return s, nil
}

// WatchDir emits a Signal for every existing .json file in dir, then for
// every subsequent create/write. The channel closes when ctx is done.
func WatchDir(ctx context.Context, dir string) (<-chan Signal, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	if err := w.Add(dir); err != nil {
		w.Close()
		return nil, err
	}
	ch := make(chan Signal, 16)
	go func() {
		defer w.Close()
		defer close(ch)
		entries, _ := os.ReadDir(dir)
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			if s, err := Parse(filepath.Join(dir, e.Name())); err == nil {
				select {
				case ch <- s:
				case <-ctx.Done():
					return
				}
			}
		}
		for {
			select {
			case <-ctx.Done():
				return
			case ev, ok := <-w.Events:
				if !ok {
					return
				}
				if ev.Op&(fsnotify.Create|fsnotify.Write) == 0 {
					continue
				}
				if !strings.HasSuffix(ev.Name, ".json") {
					continue
				}
				if s, err := Parse(ev.Name); err == nil {
					select {
					case ch <- s:
					case <-ctx.Done():
						return
					}
				}
			case _, ok := <-w.Errors:
				if !ok {
					return
				}
			}
		}
	}()
	return ch, nil
}
