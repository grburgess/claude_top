# claude_top

<!-- loop-distill:begin -->
- Go on this machine: /opt/homebrew/bin/go (NOT on default PATH). `go get`
  behind the corporate MITM proxy: GOSUMDB=off GOPROXY="file:///tmp/goproxy,direct"
  (curl-mirror golang.org/x zips into the file proxy first).
- testdata/transcripts + testdata/signals are REAL local transcripts: gitignored,
  never commit or push; agents cannot move this data (classifier quarantine) —
  populate/refresh it yourself from ~/.claude/projects.
- Run tests with /opt/homebrew/bin/go test ./... ; TUI smoke via tmux
  (scripts/soak.sh for coexistence).
<!-- loop-distill:end -->
