package registry

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

const (
	synthSessions = 1000
	synthLines    = 500
	synthLiveID   = "synth-live-0"
)

// The synthetic tree is expensive (~1000 transcripts x 500 lines), so it is
// generated once per test binary and removed in TestMain.
var (
	synthOnce sync.Once
	synthDir  string
	synthErr  error
)

func TestMain(m *testing.M) {
	code := m.Run()
	if synthDir != "" {
		os.RemoveAll(synthDir)
	}
	os.Exit(code)
}

// syntheticTree returns a projects dir holding synthSessions transcripts of
// synthLines assistant lines each. All transcripts are backdated 2h (dead)
// except synthLiveID, whose mtime is left fresh (live, no signal file).
func syntheticTree(t *testing.T) string {
	t.Helper()
	synthOnce.Do(func() {
		synthDir, synthErr = buildSyntheticTree()
	})
	if synthErr != nil {
		t.Fatalf("build synthetic tree: %v", synthErr)
	}
	return synthDir
}

func buildSyntheticTree() (string, error) {
	dir, err := os.MkdirTemp("", "claude_top_synth")
	if err != nil {
		return "", err
	}
	slugDir := filepath.Join(dir, "-Users-x-synth")
	if err := os.MkdirAll(slugDir, 0o755); err != nil {
		return "", err
	}
	var b bytes.Buffer
	for i := 0; i < synthLines; i++ {
		fmt.Fprintf(&b,
			`{"type":"assistant","message":{"id":"msg_%d","model":"claude-opus-4","usage":{"input_tokens":10,"output_tokens":20}},"timestamp":"2026-01-01T10:00:00Z"}`+"\n", i)
	}
	content := b.Bytes()
	old := time.Now().Add(-2 * time.Hour)
	for i := 0; i < synthSessions; i++ {
		id := fmt.Sprintf("synth-%04d", i)
		if i == 0 {
			id = synthLiveID
		}
		p := filepath.Join(slugDir, id+".jsonl")
		if err := os.WriteFile(p, content, 0o644); err != nil {
			return "", err
		}
		if i != 0 {
			if err := os.Chtimes(p, old, old); err != nil {
				return "", err
			}
		}
	}
	return dir, nil
}

// TestC2IndexFastStartup asserts RefreshIndex+List over the 1000-transcript
// tree completes in <1s (no transcript is tailed), then that a background
// EnsureStats round eventually yields Stats for the live session.
func TestC2IndexFastStartup(t *testing.T) {
	projects := syntheticTree(t)
	r := New(t.TempDir(), projects)

	start := time.Now()
	if err := r.RefreshIndex(); err != nil {
		t.Fatal(err)
	}
	rows := r.List(ModeAll)
	elapsed := time.Since(start)
	t.Logf("C2 RefreshIndex+List over %d transcripts: %v", len(rows), elapsed)

	if len(rows) != synthSessions {
		t.Fatalf("sessions = %d, want %d", len(rows), synthSessions)
	}
	if elapsed >= time.Second {
		t.Errorf("index time = %v, want <1s", elapsed)
	}
	var live *Session
	for _, s := range rows {
		if s.StatsReady {
			t.Fatalf("session %s parsed during index pass", s.ID)
		}
		if s.ID == synthLiveID {
			live = s
		}
	}
	if live == nil {
		t.Fatal("live session missing from index")
	}
	if live.State != StateLive {
		t.Fatalf("live session state = %v, want live (fresh mtime, no signal)", live.State)
	}

	// Background enrichment round: 4 workers over the live session plus a
	// slice of dead history, mirroring the UI's bounded pool.
	ids := []string{synthLiveID}
	for i := 1; i <= 20; i++ {
		ids = append(ids, fmt.Sprintf("synth-%04d", i))
	}
	sem := make(chan struct{}, 4)
	var wg sync.WaitGroup
	for _, id := range ids {
		wg.Add(1)
		sem <- struct{}{}
		go func(id string) {
			defer wg.Done()
			r.EnsureStats(id)
			<-sem
		}(id)
	}
	wg.Wait()

	deadline := time.Now().Add(10 * time.Second)
	for {
		var got *Session
		for _, s := range r.List(ModeAll) {
			if s.ID == synthLiveID {
				got = s
			}
		}
		if got != nil && got.StatsReady {
			if got.Stats.Turns != synthLines {
				t.Fatalf("live session Turns = %d, want %d", got.Stats.Turns, synthLines)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("live session never got Stats from EnsureStats round")
		}
		r.EnsureStats(synthLiveID) // retry if an earlier call was in flight
		time.Sleep(10 * time.Millisecond)
	}
}
