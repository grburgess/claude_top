# Project learnings · claude_top

Distilled by `loop-distill` 2026-09-01. Authoritative project layer:
global learnings sit above this file, per-loop STATE.md below it. Rows
carry `used:<n> last:<YYYY-MM-DD>`; usage — not recency — drives
prune/promote. Nothing here is ever a pass/fail gate on a loop.

## 1 Project state as-is
claude_top: Go + Bubble Tea full-screen monitor for local Claude Code sessions.
SHIPPED 2026-09-01 — 6/6 GOAL criteria, binary on PATH (~/.local/bin/claude_top →
repo build), repo private github.com/grburgess/claude_top. Design doc
docs/research/2026-09-01-claude-top.md; graph node research-plan-claude-top.

## 2 Loop registry + health

| loop | sessions | criteria | status | last touched | flags |
|---|---|---|---|---|---|
| claude-top-build | 2 | 6/6 | complete | 2026-09-01 | — |
| claude-top-v2 | 0 | 0/5 | running | 2026-09-01 | — |

## 3 Cross-loop verified facts
- iTerm2 bridge verbs all work via bare osascript (enumerate/select-by-tty/write
  newline NO/^C via string id 3/contents/create+close tab); never `index of t` in
  repeat-with · verified via live execution · 2026-09-01 · source:claude-top-build ·
  used:1 last:2026-09-01
- Subagent transcripts live at <session-id>/subagents/agent-<id>.jsonl +
  .meta.json; jsonl mtime = liveness; main-JSONL start/ack counting unreliable ·
  verified via 522-file scan + spot-check · 2026-09-01 · source:claude-top-build ·
  used:1 last:2026-09-01

## 4 Cross-loop rules
- Data flagged by a subagent security warning is session-quarantined for agents:
  hand ONE movement command to the user's shell, gitignore the data after ·
  2026-09-01 · source:claude-top-build · used:1 last:2026-09-01
- tmux-driven TUI probes: ≥1 UI-tick sleep between keypress and assert;
  capture-confirm selection before action keys · 2026-09-01 ·
  source:claude-top-build · used:1 last:2026-09-01

## 5 Loop-system project notes
- Engine Shape 1 worked well: single coupled artifact, sequential ceiling-tier
  makers + focused verifiers; orchestrator ran iTerm-automation probes in-band
  (classifier blocks them in subagent prompts combining automation + data harvest).
- This machine: go1.27 at /opt/homebrew/bin/go (not on PATH); corporate MITM
  breaks Go HTTP — GOPROXY="file:///tmp/goproxy,direct" + GOSUMDB=off for go get.
- Ceiling fable / sibling opus; 'sonnet' alias broken → normal tier = opus.

## 6 Stale / dangling / open
- 2026-09-01 · flag:stale · loop STATE "Open failures" still lists the classifier-
  block row although it was investigated→promoted to Verified facts same day ·
  evidence:STATE.md L65 vs Verified facts L33-36 · proposal: annotate resolved (loop
  complete, STATE frozen — cosmetic only)

## 7 Promotion ledger
- 2026-09-01 · gl-2026-09-01-cc-transcript-usage-per-block → global-learnings +
  mindmap · verdict:HOLDS (525-file re-verify: 2.48–2.55× inflation, 0 [1m] markers; nuance: exclude model:"<synthetic>" lines) · status:applied
- 2026-09-01 · gl-2026-09-01-flagged-artifact-quarantine → global-learnings ·
  verdict:UNPROVABLE (harness-behavioral; evidence = 3 in-session blocks, no safe
  reproduction) — row kept, marked unverified since 2026-09-01 · status:withheld
- 2026-09-01 · gl-2026-09-01-tui-probe-tick-race → global-learnings ·
  verdict:UNPROVABLE (probe-process lesson; re-running the race is nondeterministic)
  — row kept, marked unverified since 2026-09-01 · status:withheld
- 2026-09-01 · project CLAUDE.md loop-distill block (go path/proxy + quarantine
  rule) · verdict:HOLDS-candidates, user-approved 2026-09-01 · status:applied

## 8 Ideation / candidate next loops
- minor: reducer counts model:"<synthetic>" lines as turns (7 exist across 525 files; cost impact $0 — unknown-model pricing is zero) — one-line fix candidate for v2.
- claude_top v2: theming/config file; per-session token-burn timeline view; Linux/
  tmux jump backend; sanitized-fixture generator for the Cape open-source release.
- cowtop convergence: shared TUI substrate between cowtop (Textual) and claude_top
  (Bubble Tea) is duplicated effort — candidate consolidation study.

## 9 Run log
- 2026-09-01 · loops:1 (complete) · flags:1 stale (proposal) · promotions:1 pending
  verify, 2 withheld UNPROVABLE · CLAUDE.md block: proposed (gated, human present) ·
  mechanical fixes:0
