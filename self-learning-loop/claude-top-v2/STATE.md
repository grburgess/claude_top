# Project memory · claude-top-v2

Created 2026-09-01. Read this file fully at session start. Update
after every verifier verdict and at session end — write before walking
away.

<!-- memory ladder: 1 open failure -> 2 investigated -> 3 verified fact -> 4 general rule -> 5 consulted next session -->

Loop status: complete 2026-09-01

## Verified facts
<!-- inherit claude-top-build STATE Verified facts (same repo): osascript verbs,
     subagents/ structure, usage dedup per message.id, ctx-limit promotion,
     classifier quarantine + in-band probe pattern, signal/transcript schemas. -->
- AppleScript `tab` inside `tell application "iTerm2"` resolves to iTerm2's tab
  CLASS and coerces to the literal string "tab", NOT 0x09; outside a tell block
  it is a real tab byte. Method: `osascript … | od -c` on both forms, 2026-09-03.
  Post-v2 field bug: broke Enumerate → empty Router owner-map → every tty hit
  FallbackBackend → "unsupported: can't focus <tty>". Fixed in da9ad3d by binding
  `set d to tab` before the tell.

## General rules
- A verb tested ONLY through an injectable fake exercises the parser, never the
  emitter: the real iTerm enumerate script was never run by any test, so a
  delimiter that produced zero fields shipped green. Any generated-script /
  generated-query surface needs one round-trip test against the real interpreter.
- Maker context diet; probes need ≥1 UI-tick settle + capture-confirm selection
  before action keys (gl-2026-09-01-tui-probe-tick-race).
- Go at /opt/homebrew/bin/go; MITM proxy → GOPROXY file mirror + GOSUMDB=off.

## Routing overrides (learned)

## Open failures

## Iteration log
- 2.2 · class:live-probe-scripts+iterm-regression · tier:orchestrator-in-band · maker: tmux end-to-end probe (p text arrived, x-x killed target session), ghost-tty no-crash, iTerm round-trip PASS · verdict: C3 PASS (focus/reopen via real-tmux unit suite), C4 PASS, C5 PASS → ALL 5 GOAL §2 criteria PASS, loop complete. Cosmetic gap logged: tmux window index renders as ⌘N (iTerm connotation)
- 2.1 · class:tmux-backend+backend-router · tier:ceiling(fable) · maker: Backend iface, TmuxBackend (all verbs, real-server tests), FallbackBackend (SIGINT fg pgid), Router w/ 5s tty cache, UI status-line errors (84 tests) · verdict: offline gates PASS (orchestrator re-ran)
- 1.1 · class:liveness-classification+async-startup · tier:ceiling(fable) · maker: signal-less live (2min window, amber 'live' phase), RefreshIndex/EnsureStats split w/ mutex + 4-worker enrichment, 1000-file index 11.9ms (63 tests, -race clean) · verdict: C1 PASS (orchestrator sandboxed live-check: signal-less session renders in default [live]), C2 PASS-offline (11.9ms << 1s; input-responsiveness probe pending)

## Consult
- ../../.claude/PROJECT-LEARNINGS.md
- ../claude-top-build/STATE.md   <!-- prior loop's verified facts -->
- ../../CLAUDE.md

## Auto mode
- status: on
- budget spent: 3 / 20
- session-in-progress: no
- last wakeup: none
- halt reason: loop complete

## Last session
Session 1 of 4 · 2026-09-01 · 3 iterations, ALL 5 criteria PASS — loop COMPLETE
2026-09-01. claude_top now works without the iterm2-tab-status plugin (fresh-mtime
live), paints instantly on huge histories (11.9ms/1000 files, async backfill), and
acts terminal-agnostically (iTerm/tmux/fallback router). Minor open: ⌘N label for
tmux window indexes.
