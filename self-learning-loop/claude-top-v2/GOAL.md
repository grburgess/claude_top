# Goal · claude-top-v2

Created 2026-09-01. All 7 sections below are required before the
loop may start (hard gate).

## 1 · Goal
Make claude_top usable beyond this machine and this terminal: (a) sessions must
appear in the default [live] view WITHOUT the iterm2-tab-status plugin (fresh
transcript mtime = live), fixing the colleague's empty-screen; (b) first paint
must be near-instant on machines with large transcript histories — parse
live/recent first, backfill history async; (c) abstract the action bridge behind
per-session backend selection: keep iTerm2, add a full tmux backend, and degrade
gracefully (clear in-UI message, kill -INT fallback for interrupt) when no backend
can act on a session's tty. iTerm2 behavior must not regress.

## 2 · Done-criteria (grader-checkable)
| # | Criterion | How the verifier checks it |
|---|-----------|----------------------------|
| 1 | Signal-less live: with an empty signal dir and a transcript whose mtime <2min, the session classifies StateLive and renders in default [live] | registry unit test + tmux run against temp dirs (no signals) showing the row in [live] |
| 2 | Fast first paint: against a synthetic tree of ≥1000 transcripts (~500 lines each), the TUI paints a populated header+rows in <1s; full stats backfill completes async without blocking input | timed integration test (first List return <1s on that tree) + tmux capture within 1s of launch shows rows; input (h/j) responsive during backfill |
| 3 | tmux backend: all five verbs (enumerate/focus/send/interrupt/reopen-equivalent) work for a session running inside tmux | live probe: fake session in a tmux pane; focus switches to its window; send-keys text arrives; ^C kills sleep; reopen opens new window at cwd with claude -r prefilled unsubmitted |
| 4 | Backend routing + degradation: per-session backend chosen at action time (tty in tmux list-panes → tmux; tty in iTerm enumerate → iTerm; else fallback); fallback interrupt = SIGINT to the tty's foreground process group; unroutable jump/send shows a status-line message, never crashes | unit tests on router with mock backends + live probe of fallback path (session tty in neither) asserting message and no panic; interrupt fallback verified on a scratch process |
| 5 | No iTerm regression: existing suite green and live iTerm jump probe still passes | go test ./... exit 0; re-run session-1 style jump probe (scratch tab, select-by-tty) PASS |

## 3 · Verifier rubric
The verifier receives ONLY: the artifact (repo at the iteration's commit + probe
scripts it runs itself) + sections 2–3 of this file.
- Every check run by the verifier, evidence = captured command output; structural
  claims counted, not eyeballed.
- Live probes create and clean up their own scratch panes/tabs/processes. A probe
  impossible in the verifier's context → criterion UNVERIFIED with the blocker
  (orchestrator may run terminal-automation probes in-band per project note).
- Timing criteria measured, printed, and compared in-band (C2 <1s).
- An all-offline pass on C3/C4/C5-probe is PROXY-complete only.

## 4 · Budgets
- Max iterations per session: 5
- Max sessions before forced escalation: 4
- Auto mode: on — both-bounded: self-continue across sessions + run non-stop within. See STATE.md § Auto mode.
- Total budget ceiling: 20 total iterations — a hard stop across ALL sessions; auto mode never continues past it.
- Stop-and-notify triggers: ceiling hit | session cap | 2× no-progress | irreversible/outward action (publish/delete/submit/send) | classifier-block with no §6 sibling | subagent null-twice | repeated maker thrash | loop complete. On any trigger: HALT + PushNotification, no further wakeup.

## 5 · Escalation rule
Write blocker to STATE.md Open failures; end session with summary and a concrete
question for the user. Terminal-automation permission denials escalate immediately
with the exact error.

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

Note: 'sonnet' alias broken in this environment — normal tier resolves to opus.

Seeded task classes: liveness-classification, async-startup, tmux-backend,
backend-router, live-probe-scripts, iterm-regression
Learned promotions override the table above — see STATE.md § Routing overrides.

Note: the em-dash characters (—, →) and the §, · symbols must be preserved exactly.

## 7 · Engine
Shape 1 subagent loop — same single coupled artifact as claude-top-build; sequential
makers, integration-level criteria.
Isolation: worktree
