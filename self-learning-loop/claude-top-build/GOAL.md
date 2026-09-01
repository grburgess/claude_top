# Goal · claude-top-build

Created 2026-09-01. All 7 sections below are required before the
loop may start (hard gate).

## 1 · Goal
Build claude_top: a full-screen Go + Bubble Tea/Lipgloss terminal monitor for all
local Claude Code sessions, run in an iTerm floating hotkey window. It merges
iterm2-tab-status signal files (`~/.cache/claude-tab-status/*.json`:
status/tty/pid/cwd) with incrementally-tailed transcript JSONLs
(`~/.claude/projects/<cwd-slug>/<session-id>.jsonl`) to render rich per-session rows
(status, ai-title, project, branch, model+effort, context gauge, cost/token
sparkline, skills, workflows, subagent count, MCP servers, current tool call, todos,
permission mode) plus a detail pane with live transcript tail. Actions via an
osascript AppleScript bridge: enter jumps to the session's iTerm2 tab, k interrupts,
p sends a prompt; closed sessions reopen a tab at their cwd with `claude -r <id>`
prefilled (never auto-sent); h cycles live → live+recent → history views.
claude_top is strictly read-only toward the signal dir and transcripts.
Approved design: docs/research/2026-09-01-claude-top.md.

## 2 · Done-criteria (grader-checkable)
| # | Criterion | How the verifier checks it |
|---|-----------|----------------------------|
| 1 | Binary builds and lists live sessions: pointed at a fixture signal dir + transcript tree (env-overridable paths), the list renders each session's status, ai-title, project, model; a signal file written mid-run appears as a row within 2s | `go build ./...` exit 0; teatest (or scripted run in tmux) against fixtures under `testdata/`; timed test asserts new-signal row <2s |
| 2 | Jump works: on a live row, enter activates the iTerm2 tab owning that session's tty; on a closed row, opens a new tab cd'd to the session cwd with `claude -r <id>` typed but not submitted | scripted live probe: osascript creates ≥2 windows/≥5 scratch tabs, records ttys; invoke bridge verbs directly + via the app; osascript reads back current session tty and (closed case) the tab's cwd/input line. Documented refutation arm: if iTerm2 scripting denies an operation, the criterion passes with the denial captured and a fallback documented |
| 3 | Detail pane fields are correct: for a real recorded session, context-token count, cost, skills list, MCP servers, subagent count match an independent reference parse of the same JSONL | `claude_top --inspect <session-id>` emits JSON; verifier compares against its own python3 parse of the transcript (counts, not eyeballs); tolerance ±5% on cost, exact on counts/lists |
| 4 | Actions + modes: h cycles live→live+recent→history; k delivers ^C to the target session's tty; p delivers typed text that arrives in the target session | teatest for h-cycle state; live probe: scratch tab runs `sleep 999` → k → process gone; scratch tab runs `cat` → p "hello" → osascript reads tab contents contains "hello" |
| 5 | Reducers never crash and survive schema drift: test suite green on fixture transcripts from ≥2 Claude Code versions plus a corrupted/unknown-lines fixture | `go test ./...` exit 0; fixtures include ≥2 distinct `version` values and a garbage-line file; any panic = fail |
| 6 | Read-only coexistence: claude_top never opens signal-dir or transcript files for writing, and a ≥30-min soak beside the running iterm2-tab-status adapter shows adapter signal files still refreshing | code audit: grep for write-mode opens against those paths (none outside testdata); soak script: run claude_top 30 min with ≥1 live session, assert signal-file mtimes keep advancing and dir contents are modified only by hook pids |

## 3 · Verifier rubric
The verifier receives ONLY: the artifact (repo at the iteration's commit + any probe
scripts/outputs it is told to run) + sections 2–3 of this file.
- Pass per criterion requires the exact check in the table, run by the verifier
  itself (commands + captured output), never the maker's claims.
- Structural claims (counts, "field absent", "N rows") must be parsed/counted, not
  eyeballed.
- Criterion 2/4 live probes run on this machine against real iTerm2; scratch
  windows/tabs must be created and cleaned up by the probe script. If a live probe
  is impossible in the verifier's context, return the criterion as UNVERIFIED with
  the blocker — never infer pass from code reading.
- Criterion 3 tolerance: cost ±5%; token counts exact; list-valued fields compared
  as sets.
- An all-offline pass on criteria 2/4/6 is PROXY-complete only — final pass
  requires the live arms.

## 4 · Budgets
- Max iterations per session: 5
- Max sessions before forced escalation: 5
- Auto mode: on — both-bounded: self-continue across sessions + run non-stop within. See STATE.md § Auto mode.
- Total budget ceiling: 25 total iterations — a hard stop across ALL sessions; auto mode never continues past it.
- Stop-and-notify triggers: ceiling hit | session cap | 2× no-progress | irreversible/outward action (publish/delete/submit/send) | classifier-block with no §6 sibling | subagent null-twice | repeated maker thrash | loop complete. On any trigger: HALT + PushNotification, no further wakeup.

## 5 · Escalation rule
Write blocker to STATE.md Open failures; end session with summary and a concrete
question for the user. Live-probe failures that implicate macOS/iTerm2 permissions
(automation consent, hotkey-window focus) escalate immediately with the exact
denial captured — the user must grant consent, the loop cannot.

## 6 · Model routing
Ceiling (auto-detected at INIT, not asked): fable
Alias ladder: haiku < sonnet < opus < fable
Classifier-block sibling: opus

Router — orchestrator tags each work item (difficulty + task class):
| Tag    | Tier |
|--------|------|
| hard   | ceiling (omit model → inherit session model) |
| normal | one tier below ceiling (clamp at haiku) |
| bulk   | cheapest fast tier (haiku; sonnet if the class needs it) |
| check  | cheapest-that-can-judge (haiku); ceiling for hard rubrics |

Note: 'sonnet' alias is broken in this environment (user CLAUDE.md) — normal tier
resolves to opus, bulk to haiku.

Seeded task classes: collector-fsnotify, transcript-reducers, session-model,
iterm-bridge-osascript, ui-list-view, ui-detail-pane, live-probe-scripts,
fixture-harvest, soak-coexistence
Learned promotions override the table above — see STATE.md § Routing overrides.

Note: the em-dash characters (—, →) and the §, · symbols must be preserved exactly.

## 7 · Engine
Shape 1 subagent loop — single tightly-coupled artifact (one Go app) refined in
place; done-criteria are integration-level, no independent N-item fan-out.
Isolation: worktree
