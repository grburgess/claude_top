package signalfile

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestParse(t *testing.T) {
	dir := t.TempDir()
	tests := []struct {
		name    string
		content string
		wantErr bool
		want    Signal
	}{
		{
			name: "valid",
			content: `{"session_id":"abc","type":"running","message":"hi","project":"p",` +
				`"cwd":"/tmp","tty":"/dev/ttys001","pid":"123","ts":"1700000000","activity":"Bash"}`,
			want: Signal{
				SessionID: "abc", Type: "running", Message: "hi", Project: "p",
				Cwd: "/tmp", TTY: "/dev/ttys001", Pid: 123,
				Ts: time.Unix(1700000000, 0), Activity: "Bash",
			},
		},
		{
			name:    "missing session_id",
			content: `{"type":"idle","ts":"1700000000"}`,
			wantErr: true,
		},
		{
			name:    "missing optional fields",
			content: `{"session_id":"xyz"}`,
			want:    Signal{SessionID: "xyz"},
		},
		{
			name:    "bad pid and ts tolerated",
			content: `{"session_id":"q","pid":"notanum","ts":"garbage"}`,
			want:    Signal{SessionID: "q"},
		},
		{
			name:    "garbage",
			content: `this is not json`,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := writeFile(t, dir, tt.name+".json", tt.content)
			got, err := Parse(p)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("want error, got %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("got %+v want %+v", got, tt.want)
			}
		})
	}
}

func TestParseMissingFile(t *testing.T) {
	if _, err := Parse(filepath.Join(t.TempDir(), "nope.json")); err == nil {
		t.Fatal("want error for missing file")
	}
}
