package transcript

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRealFixtures runs the reducer over any transcripts placed under
// testdata/transcripts/. Skipped when the directory does not exist.
func TestRealFixtures(t *testing.T) {
	dir := filepath.Join("testdata", "transcripts")
	if _, err := os.Stat(dir); err != nil {
		t.Skip("no testdata/transcripts dir")
	}
	paths, err := filepath.Glob(filepath.Join(dir, "*.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Skip("no fixtures")
	}
	for _, p := range paths {
		p := p
		t.Run(filepath.Base(p), func(t *testing.T) {
			var st Stats
			var r Reader
			if err := r.Tail(p, &st); err != nil {
				t.Fatalf("Tail error: %v", err)
			}
			if strings.HasPrefix(filepath.Base(p), "garbage") {
				return // garbage must simply not error or panic
			}
			if st.Model == "" {
				t.Errorf("Model empty for real fixture")
			}
			if st.Turns == 0 {
				t.Errorf("Turns == 0 for real fixture")
			}
		})
	}
}
