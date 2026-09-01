#!/usr/bin/env bash
# C6 coexistence soak: run claude_top against the REAL default dirs for
# DURATION_MIN minutes and prove it never opens a write FD there, while the
# tab-status adapter keeps writing signal files beside us.
set -uo pipefail

DURATION_MIN="${1:-30}"
TMUX_BIN="${TMUX_BIN:-/opt/homebrew/bin/tmux}"
SESSION="claude_top_soak_$$"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN="$ROOT/claude_top"

SIGNAL_DIR="${CLAUDE_TOP_SIGNAL_DIR:-$HOME/.cache/claude-tab-status}"
PROJECTS_DIR="${CLAUDE_TOP_PROJECTS_DIR:-$HOME/.claude/projects}"

cleanup() {
	"$TMUX_BIN" kill-session -t "$SESSION" 2>/dev/null
}
trap cleanup EXIT INT TERM

[ -x "$BIN" ] || { echo "SOAK FAIL: $BIN not built (go build -o claude_top ./cmd/claude_top)"; exit 1; }
[ -x "$TMUX_BIN" ] || { echo "SOAK FAIL: tmux not found at $TMUX_BIN (set TMUX_BIN)"; exit 1; }
[ -d "$SIGNAL_DIR" ] || { echo "SOAK FAIL: signal dir $SIGNAL_DIR missing"; exit 1; }

mtimes() { # one "path epoch" line per signal file
	find "$SIGNAL_DIR" -maxdepth 1 -name '*.json' -exec stat -f '%N %m' {} \; 2>/dev/null | sort
}

echo "soak: ${DURATION_MIN}m, signal dir $SIGNAL_DIR, projects dir $PROJECTS_DIR"
"$TMUX_BIN" new-session -d -s "$SESSION" -x 200 -y 50 "$BIN"
sleep 2
PID="$("$TMUX_BIN" list-panes -t "$SESSION" -F '#{pane_pid}' 2>/dev/null | head -1)"
[ -n "${PID:-}" ] || { echo "SOAK FAIL: claude_top did not start in tmux"; exit 1; }
echo "soak: claude_top pid $PID"

BASELINE="$(mtimes)"
END=$(( $(date +%s) + DURATION_MIN * 60 ))
CHECKS=0
WRITE_FDS=0

while [ "$(date +%s)" -lt "$END" ]; do
	sleep 60
	kill -0 "$PID" 2>/dev/null || { echo "SOAK FAIL: claude_top died after $CHECKS checks"; exit 1; }
	CHECKS=$(( CHECKS + 1 ))
	# FD mode is the 4th lsof column (e.g. "5u", "7w"); flag any w/u under our dirs
	HITS="$(lsof -p "$PID" 2>/dev/null \
		| awk -v s="$SIGNAL_DIR" -v p="$PROJECTS_DIR" \
			'$4 ~ /[wu]/ && (index($0, s) || index($0, p)) {print}')"
	if [ -n "$HITS" ]; then
		WRITE_FDS=$(( WRITE_FDS + $(printf '%s\n' "$HITS" | wc -l | tr -d ' ') ))
		echo "soak: WRITE FD at check $CHECKS:"
		printf '%s\n' "$HITS"
	fi
	mtimes >/dev/null
done

FINAL="$(mtimes)"
CHANGED=0
[ "$BASELINE" != "$FINAL" ] && CHANGED=1

if [ "$WRITE_FDS" -gt 0 ]; then
	echo "SOAK FAIL: write FDs=$WRITE_FDS over $CHECKS checks, signal mtimes advanced=$CHANGED"
	exit 1
fi
if [ "$CHANGED" -eq 0 ]; then
	echo "SOAK INCONCLUSIVE: write FDs=0 over $CHECKS checks, but no signal mtime advanced (no live claude session?)"
	exit 0
fi
echo "SOAK PASS: write FDs=0 over $CHECKS checks, signal mtimes advanced"
