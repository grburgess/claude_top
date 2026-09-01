# Project memory · claude-top-v2

Created 2026-09-01. Read this file fully at session start. Update
after every verifier verdict and at session end — write before walking
away.

<!-- memory ladder: 1 open failure -> 2 investigated -> 3 verified fact -> 4 general rule -> 5 consulted next session -->

Loop status: running

## Verified facts
<!-- inherit claude-top-build STATE Verified facts (same repo): osascript verbs,
     subagents/ structure, usage dedup per message.id, ctx-limit promotion,
     classifier quarantine + in-band probe pattern, signal/transcript schemas. -->

## General rules
- Maker context diet; probes need ≥1 UI-tick settle + capture-confirm selection
  before action keys (gl-2026-09-01-tui-probe-tick-race).
- Go at /opt/homebrew/bin/go; MITM proxy → GOPROXY file mirror + GOSUMDB=off.

## Routing overrides (learned)

## Open failures

## Iteration log

## Consult
- ../../.claude/PROJECT-LEARNINGS.md
- ../claude-top-build/STATE.md   <!-- prior loop's verified facts -->
- ../../CLAUDE.md

## Auto mode
- status: on
- budget spent: 0 / 20
- session-in-progress: yes
- last wakeup: none
- halt reason: none

## Last session
None yet — session 1 pending. Work order: (1) signal-less live + async startup
(colleague-blocking), (2) backend router + tmux backend, (3) live probes + iTerm
regression.
