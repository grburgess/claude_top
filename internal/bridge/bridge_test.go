package bridge

import (
	"reflect"
	"strings"
	"testing"
)

func TestEscapeOSA(t *testing.T) {
	got := escapeOSA(`say "hi" c:\path`)
	want := `say \"hi\" c:\\path`
	if got != want {
		t.Errorf("escapeOSA = %q, want %q", got, want)
	}
}

func TestParseEnumerate(t *testing.T) {
	out := "1\t1\tS-A\t/dev/ttys001\tzsh\n1\t2\tS-B\t/dev/ttys002\tclaude\n\n"
	got := parseEnumerate(out)
	want := []TermSession{
		{WindowID: "1", TabIndex: 1, SessionID: "S-A", TTY: "/dev/ttys001", Name: "zsh"},
		{WindowID: "1", TabIndex: 2, SessionID: "S-B", TTY: "/dev/ttys002", Name: "claude"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v want %+v", got, want)
	}
}

func TestOSAScriptsGenerated(t *testing.T) {
	var scripts []string
	o := &OSAITerm{run: func(s string) (string, error) {
		scripts = append(scripts, s)
		return "", nil
	}}
	if err := o.SendText("/dev/ttys001", `echo "x"`, false); err != nil {
		t.Fatal(err)
	}
	if err := o.SendText("/dev/ttys001", "ls", true); err != nil {
		t.Fatal(err)
	}
	if err := o.Interrupt("/dev/ttys001"); err != nil {
		t.Fatal(err)
	}
	if err := o.FocusTTY("/dev/ttys001"); err != nil {
		t.Fatal(err)
	}
	if err := o.NewTabAt("/tmp/x", "claude --resume"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(scripts[0], `write s text "echo \"x\"" newline NO`) {
		t.Errorf("unsubmitted script wrong:\n%s", scripts[0])
	}
	if !strings.Contains(scripts[1], `write s text "ls"`) || strings.Contains(scripts[1], "newline NO") {
		t.Errorf("submitted script wrong:\n%s", scripts[1])
	}
	if !strings.Contains(scripts[2], "write s text (string id 3) newline NO") {
		t.Errorf("interrupt script wrong:\n%s", scripts[2])
	}
	for _, frag := range []string{"select t", "select s", "activate"} {
		if !strings.Contains(scripts[3], frag) {
			t.Errorf("focus script missing %q:\n%s", frag, scripts[3])
		}
	}
	if !strings.Contains(scripts[4], "create tab with default profile") ||
		!strings.Contains(scripts[4], `write text "cd /tmp/x"`) ||
		!strings.Contains(scripts[4], `write text "claude --resume" newline NO`) {
		t.Errorf("new tab script wrong:\n%s", scripts[4])
	}
	// Enumerate must never use `index of t`.
	if strings.Contains(enumerateScript, "index of t") {
		t.Error("enumerate script must use a manual tab counter, not `index of t`")
	}
}

func TestMockITermRecords(t *testing.T) {
	m := &MockITerm{Sessions: []TermSession{{TTY: "/dev/ttys009"}}}
	ss, err := m.Enumerate()
	if err != nil || len(ss) != 1 {
		t.Fatalf("Enumerate = %v, %v", ss, err)
	}
	_ = m.FocusTTY("/dev/ttys009")
	_ = m.SendText("/dev/ttys009", "hi", false)
	_ = m.Interrupt("/dev/ttys009")
	_ = m.NewTabAt("/tmp", "claude")
	want := []string{
		"Enumerate()",
		"FocusTTY(/dev/ttys009)",
		`SendText(/dev/ttys009,"hi",false)`,
		"Interrupt(/dev/ttys009)",
		`NewTabAt(/tmp,"claude")`,
	}
	if !reflect.DeepEqual(m.Calls, want) {
		t.Errorf("Calls = %v, want %v", m.Calls, want)
	}
}
