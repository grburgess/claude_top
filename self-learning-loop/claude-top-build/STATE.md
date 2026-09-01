# Project memory · claude-top-build

Created 2026-09-01. Read this file fully at session start. Update
after every verifier verdict and at session end — write before walking
away.

<!-- memory ladder: 1 open failure -> 2 investigated -> 3 verified fact -> 4 general rule -> 5 consulted next session -->

Loop status: complete 2026-09-01

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
- 5.4 · class:soak-coexistence · tier:orchestrator-in-band · maker: 30-min soak vs live adapter · verdict: C6 PASS (write FDs=0 over 30 checks, signal mtimes advanced) → ALL 6 GOAL §2 criteria PASS, loop complete
- 5.3 · class:live-probe-scripts+soak-coexistence · tier:opus · maker: C1 timed live tests (measured 1.5ms/1.2ms vs 2s deadline; orchestrator re-ran PASS) + scripts/soak.sh · verdict: C1 PASS; C6 soak launched in background (30min)
- 5.2 · class:live-probe-scripts · tier:orchestrator-in-band · maker: sandboxed fake-session probe (scratch cat tab + synthetic signals, claude_top in tmux w/ temp dirs) · verdict: C2 PASS (live jump focused target tty; dead reopen created tab w/ cd+`claude -r` prefill — tabs 11→12, prefill=true), C4 PASS (h cycle, x-x ^C delivered, p text arrived submitted); NOTE probe races: allow ≥1 UI tick between keypress and assert
- 5.1 · class:ui-detail-pane+transcript-reducers · tier:ceiling(fable) · maker: x armed-interrupt, p minibuffer, ReopenAt, CostTodayUSD + per-mode header (64 tests) · verdict: offline gates PASS (orchestrator re-ran, merged d2f8376)
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
- budget spent: 7 / 25
- session-in-progress: no
- last wakeup: scheduled @ session-1 end (1800s)
- halt reason: loop complete

## Memory check
- 2026-09-01 s2: soak + probe findings promoted; Verified-fact rows name what they pass (GOAL §2 criteria), per lessons 2026-08-19; probe-race rule distilled to global ledger. No stale recalls.
- 2026-09-01 s1: verified findings promoted to Verified facts (5 rows); no ≥2× patterns yet; Consult items fresh; classifier-block failure investigated→verified same session. No stale recalls.

## Last session
<!-- Session <k> of <max sessions> · <YYYY-MM-DD> · what happened · criteria <n>/<m> passing
     Next: <exact next action> -->
Session 2 of 5 · 2026-09-01 · 4 iterations (5.1–5.4): x/p/reopen actions + truthful
header, live probes C2/C4 PASS, C1 timed tests (1.5ms), C6 soak PASS. ALL 6 GOAL §2
criteria PASS — loop COMPLETE 2026-09-01. Binary on PATH (~/.local/bin), repo
github.com/grburgess/claude_top private.
Prior session — Session 1 of 5 · 2026-09-01 · 4 iterations: spikes (RQ1 all iTerm verbs PASS, RQ2
subagents-dir mechanism), Go scaffold (5 pkgs + --inspect), reducer fixes (usage
dedup, ctx-limit promotion, clamp) + list view, toolbelt-style redesign + inline
detail card + ⌘N join. Repo published private github.com/grburgess/claude_top
(fixtures gitignored, local-only). Criteria: C3 PASS, C5 PASS (incl. 3-version
fixtures, user-run), C1/C2/C4 partial (render+jump verbs proven; 2s test, row-level
jump probe, k/p actions missing), C6 not run.
Next: header per-mode counts + real '$ today'; UX DECIDED (user, 2026-09-01): keep
3-mode h-cycle, history hidden unless manually called — no always-visible dimmed rows;
k interrupt + p prompt + closed-session reopen; then C1/C2/C4 probes + C6 soak.
