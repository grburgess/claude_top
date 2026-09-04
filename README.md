# claude_top

A full-screen terminal monitor for Claude Code sessions, in the spirit of `top`. While iTerm2's toolbelt can surface per-session status, it is cramped, read-only, and blind to the rich state a session actually carries; here we render that state — model, effort, context consumption, cost, loaded skills, MCP servers, live subagents, per-turn token history — as live, scrollable session blocks, and we make the list actionable: hitting enter jumps to the session's iTerm2 tab.

Built in Go on Bubble Tea and Lipgloss. Intended to run in an iTerm2 floating hotkey window, though any terminal works.

## What it shows

Each session renders as a compact block: a colored status dot (⚡ working, 💤 idle, 🔴 attention), the session's AI-generated title, its tab shortcut (⌘N), the state word, and a dimmed snippet of the last assistant message. Selecting a block expands it in place into a detail card:

- context gauge — tokens against the model's context limit (200k/1M autodetected), gradient bar
- cost and token totals, deduplicated per API message (Claude Code writes one transcript line per content block, each repeating cumulative usage; naive summation inflates cost roughly 2–3×)
- skills invoked, MCP servers touched, permission mode
- subagents — live/total counts and names, derived from `subagents/*.meta.json` and transcript mtimes
- a per-turn output-token sparkline over the last 40 turns

The header aggregates counts by state and cost; `h` cycles three views — live, live+recent (30 min), and the full session history on disk.

## Data sources

claude_top writes nothing; it merges two read-only sources:

1. **Signal files** from the [iterm2-tab-status](https://github.com/jaspersui/claude-code-iterm2-tab-status) plugin's hooks (`~/.cache/claude-tab-status/*.json`) — coarse status, tty, pid, cwd. Install that plugin for live status; without it, sessions are classified from transcript mtimes alone.
2. **Transcripts** (`~/.claude/projects/<cwd-slug>/<session-id>.jsonl`) — tailed incrementally with monotonic byte offsets; malformed or unknown lines are skipped, never fatal.

**A session is live while its terminal is open**, not while it is busy: the signal file records the tab's `login` pid, which exits with the tab, so liveness is a `kill(pid, 0)` probe rather than a timestamp heuristic. A session idle for hours stays live and enter jumps to it; only a session whose terminal is gone falls to recent (transcript under 30 min) or dead, and only there does enter open a fresh tab with `claude -r <id>`. Where no pid is available — a signal-less machine — the mtime heuristics still apply, and enter declines to act rather than risk a duplicate of a session that is already running.

Tab jumping, interrupt, and prompt dispatch go through plain `osascript` against iTerm2's AppleScript dictionary — no Python API runtime, no persistent connection.

## Install

Requires Go ≥ 1.22 and macOS with iTerm2 (the monitor itself runs anywhere; the jump/interrupt actions are iTerm2-specific).

**Prerequisite for the live view:** the default `[live]` mode is driven by signal files from the [iterm2-tab-status](https://github.com/jaspersui/claude-code-iterm2-tab-status) Claude Code plugin — install it first (`/plugin install iterm2-tab-status` in Claude Code, then its `/iterm2-tab-status:setup`). Without it, sessions lack status signals and the default view appears **empty**; your sessions are still there under `h` → `[all]`, classified from transcript mtimes alone.

**Known first-launch cost:** startup currently parses every transcript under `~/.claude/projects/` before first paint — on a machine with a long Claude history this can take tens of seconds. One-time per launch; lazy/async loading is planned.

```sh
git clone https://github.com/grburgess/claude_top.git
cd claude_top
go build -o claude_top ./cmd/claude_top
ln -sf "$PWD/claude_top" ~/.local/bin/claude_top   # or anywhere on PATH
```

Behind a TLS-intercepting corporate proxy, Go's module fetch may fail where curl succeeds; `GOSUMDB=off GOPROXY=direct go build ...` is usually sufficient, and the vendored `go.sum` is populated from the official proxy.

## Usage

```sh
claude_top
```

| Key | Action |
|-----|--------|
| `j`/`k`, `↑`/`↓` | move selection (selected block expands) |
| `enter` | jump to the session's iTerm2 tab |
| `h` | cycle view: live → live+recent → all |
| `r` | force refresh |
| `q` | quit |

Environment overrides: `CLAUDE_TOP_SIGNAL_DIR`, `CLAUDE_TOP_PROJECTS_DIR`.

`claude_top --inspect <session-id>` dumps a session's full reduced stats as JSON — useful for verifying the reducer against a transcript directly.

## Development

```sh
go test ./...
```

The reducer test suite covers usage deduplication, context-limit promotion, negative-token clamping, offset monotonicity, and garbage-line tolerance; `testdata/transcripts/` fixtures are deliberately **not** committed — real transcripts contain prompts and tool output, and the fixture-driven tests skip when the directory is absent. Populate locally from your own `~/.claude/projects/` if needed.

The build itself is run as a self-learning maker/verifier loop; its state lives under `self-learning-loop/` and the research plan under `docs/research/`.

## Status

Working: list view, detail expansion, jump-to-tab, view cycling, `--inspect`. Planned: `k` interrupt and `p` prompt-dispatch from the list, closed-session reopen (`claude -r` prefill), per-mode header counts, a corrected daily-cost aggregate, and a coexistence soak. We note the transcript schema is undocumented and drifts across Claude Code versions; the reducers are deliberately tolerant, and fixtures spanning multiple versions gate regressions.

Private for now; a Cape release may follow.
