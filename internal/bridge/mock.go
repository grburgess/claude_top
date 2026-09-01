package bridge

import "fmt"

// MockITerm records calls for tests.
type MockITerm struct {
	Sessions []TermSession // returned by Enumerate
	Calls    []string      // recorded call descriptions
	Err      error         // returned by every method when set
}

var _ ITerm = (*MockITerm)(nil)

func (m *MockITerm) record(format string, args ...any) {
	m.Calls = append(m.Calls, fmt.Sprintf(format, args...))
}

func (m *MockITerm) Enumerate() ([]TermSession, error) {
	m.record("Enumerate()")
	return m.Sessions, m.Err
}

func (m *MockITerm) FocusTTY(tty string) error {
	m.record("FocusTTY(%s)", tty)
	return m.Err
}

func (m *MockITerm) SendText(tty, text string, submit bool) error {
	m.record("SendText(%s,%q,%v)", tty, text, submit)
	return m.Err
}

func (m *MockITerm) Interrupt(tty string) error {
	m.record("Interrupt(%s)", tty)
	return m.Err
}

func (m *MockITerm) NewTabAt(cwd, prefill string) error {
	m.record("NewTabAt(%s,%q)", cwd, prefill)
	return m.Err
}
