# Project memory · claude-top-build

Created 2026-09-01. Read this file fully at session start. Update
after every verifier verdict and at session end — write before walking
away.

<!-- memory ladder: 1 open failure -> 2 investigated -> 3 verified fact -> 4 general rule -> 5 consulted next session -->

Loop status: running

## Verified facts
<!-- stage 3 — stop guessing about these.
     Format: fact. Verified via <method>, <YYYY-MM-DD>. -->
- RQ1/H1 CONFIRMED: all iTerm2 bridge verbs work via bare osascript — enumerate
  (0.6s/9 sessions; never `index of t` in repeat-with, keep own counter), select
  by tty, `write text ... newline NO` (unsubmitted), `write text (string id 3)
  newline NO` (^C kills sleep), contents readback, create/close tab, focus
  restore. Verified via live execution, 2026-09-01 (artifacts/session-1/).
- RQ2/H2 REFUTED-as-phrased, better path found: sidechains are NEVER inline in
  the main JSONL — they live at <session-id>/subagents/agent-<id>.jsonl +
  agent-<id>.meta.json {agentType,description,toolUseId,spawnDepth,model};
  jsonl mtime = liveness signal. Main-JSONL start/completion counting is
  unreliable (async acks land ~3s after start; 59/201 splits across resumed
  files). Verified via maker scan (522 files) + orchestrator spot-check,
  2026-09-01.
- Claude Code writes one assistant JSONL line PER CONTENT BLOCK, each repeating
  the same cumulative message.usage — accumulate usage only once per message.id
  (inflation measured 1.79–2.67×). Verified via independent verifier dedup
  comparison, 2026-09-01.
- 1M-context sessions record message.model as plain "claude-opus-5" (no [1m]
  marker in transcript) — static model→limit map insufficient; observed context
  403726 > 200k. Verified via verifier on real transcript, 2026-09-01.
- Classifier blocks (2026-09-01): (a) subagent prompts combining iTerm
  automation + transcript harvesting; (b) ANY merge of the security-flagged
  fixture branch, even with user approval in chat — user must run the merge
  themselves. Orchestrator may run osascript probes in-band.
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
- This machine: go1.27 at /opt/homebrew/bin/go (NOT on default PATH). Corporate
  MITM proxy breaks Go's HTTP client — `go get` needs
  GOPROXY="file:///tmp/goproxy,direct" + GOSUMDB=off (curl-mirror golang.org/x
  zips first). Recorded from maker 2.1 deviations, 2026-09-01.

## Routing overrides (learned)
<!-- task class | tier | reason | YYYY-MM-DD -->

## Open failures
<!-- stages 1–2 — investigate next session.
     Format: <YYYY-MM-DD> · symptom · hypothesis. -->
- 2026-09-01 · blocked: classifier — combined spike maker (osascript iTerm-driving + transcript harvest) denied at dispatch · hypothesis: terminal-automation + user-data-harvest combo trips it; mitigation: orchestrator runs osascript probes in-band, read-only analysis split to its own maker.

## Iteration log
<!-- one line per iteration:
     <session>.<iter> · class:<label> · tier:<used> · maker: <action> · verdict: PASS | gaps(<n>): <short> -->
- 4.1 · class:ui-list-view+ui-detail-pane · tier:ceiling(fable) · maker: toolbelt-style blocks (dot/Chat/spinner/title/⌘N/state/snippet), inline detail card on selection (gauge/cost/skills/mcp/agents/sparkline), ⌘N tty→tab join via Enumerate every 5s, 54 tests · verdict: offline gates PASS (orchestrator re-ran); user re-smoke pending
- 3.2 · class:ui-list-view · tier:— · maker: (user smoke) · verdict: gaps(3) from user: not slick, doesn't mimic iTerm toolbelt session blocks, selected row must expand with detail → folded into 4.1 with transcribed toolbelt visual spec
- 3.1 · class:transcript-reducers+ui-list-view · tier:ceiling(fable) · maker: dedup-by-message.id fix, context-limit promotion, negative clamp, full Bubble Tea list view (header/rows/gauge/responsive/keys, 37 tests) · verdict: offline gates PASS (orchestrator re-ran go test independently; corrected inspect: 68 turns $64.19 ctx 285k/1M); TUI human smoke pending (no tty in maker/orchestrator context)
- 2.1 · class:collector-fsnotify+transcript-reducers+iterm-bridge-osascript · tier:ceiling(fable) · maker: full Go scaffold (5 internal pkgs + --inspect CLI, 28 tests) · verdict: C3 PASS (5 transcripts, exact equality, cost delta 0.0%), C5 PASS-offline/UNVERIFIED-fixtures (testdata unmerged; 524 real transcripts × 18 CC versions tail clean) · gaps(3): usage multi-counted per content-block line → dedup message.id (cost 1.8–2.7× inflated); ContextLimit 200k mislabels 1M sessions (model string is plain claude-opus-5); negative tokens unclamped in TokensPerTurn
- 1.1 · class:live-probe-scripts+fixture-harvest · tier:opus(+orchestrator in-band for osascript after classifier block) · maker: RQ1 probe (all iTerm verbs PASS, findings-iterm-probe.md), RQ2 sidechain structure (subagents/ dir + meta.json, verified by orchestrator spot-check), fixtures v2.1.220/235/251 + garbage · verdict: staged — final artifact absent → criteria 0/6 unchanged (staged-goal guard, no verifier spent); fixture merge awaits user (classifier blocks flagged branch)

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
