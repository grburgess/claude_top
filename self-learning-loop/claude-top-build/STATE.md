# Project memory · claude-top-build

Created 2026-09-01. Read this file fully at session start. Update
after every verifier verdict and at session end — write before walking
away.

<!-- memory ladder: 1 open failure -> 2 investigated -> 3 verified fact -> 4 general rule -> 5 consulted next session -->

Loop status: running

## Verified facts
<!-- stage 3 — stop guessing about these.
     Format: fact. Verified via <method>, <YYYY-MM-DD>. -->
- Signal files (`~/.cache/claude-tab-status/<session-id>.json`) carry session_id,
  type(running|idle|attention), message, project, cwd, tty, pid(login shell), ts;
  written by hook.sh on UserPromptSubmit/Notification. Verified via plugin source
  read, 2026-09-01.
- Transcript JSONLs carry per-line: type, message.model, message.usage (in/out,
  cache_creation, cache_read, thinking), effort, ai-title, mode, permission-mode,
  attributionSkill, attributionMcpServer/Tool, isSidechain, gitBranch, version,
  slug, cwd. Verified via python key-survey of a live transcript, 2026-09-01.

## General rules
<!-- stage 4 — consult before re-deriving.
     Promoted when a pattern is verified ≥2×. -->
- Maker context diet (global lesson 2026-07-02): inline needed facts into maker
  prompts; makers read only files they will edit; pipe build/test output through
  tail.
- Incremental JSONL tailing: byte offsets must be monotonic; never re-read whole
  files per tick ([[learning-textual-tui-testing-gotchas]] analogue).

## Routing overrides (learned)
<!-- task class | tier | reason | YYYY-MM-DD -->

## Open failures
<!-- stages 1–2 — investigate next session.
     Format: <YYYY-MM-DD> · symptom · hypothesis. -->
- 2026-09-01 · blocked: classifier — combined spike maker (osascript iTerm-driving + transcript harvest) denied at dispatch · hypothesis: terminal-automation + user-data-harvest combo trips it; mitigation: orchestrator runs osascript probes in-band, read-only analysis split to its own maker.

## Iteration log
<!-- one line per iteration:
     <session>.<iter> · class:<label> · tier:<used> · maker: <action> · verdict: PASS | gaps(<n>): <short> -->

## Consult
<!-- skills/files to read at session start, one per line -->
- ../../.claude/PROJECT-LEARNINGS.md   <!-- project layer: cross-loop facts, rules, loop-system notes -->
- ../../docs/research/2026-09-01-claude-top.md   <!-- approved design + RQ spikes -->

## Auto mode
- status: on
- budget spent: 0 / 25
- session-in-progress: yes
- last wakeup: none
- halt reason: none

## Last session
<!-- Session <k> of <max sessions> · <YYYY-MM-DD> · what happened · criteria <n>/<m> passing
     Next: <exact next action> -->
None yet — session 1 pending. First work items: RQ1 osascript tty→session probe,
RQ2 sidechain disk-structure inspection, fixture harvest from real transcripts.
