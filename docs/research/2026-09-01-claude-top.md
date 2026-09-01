# claude_top — research plan + approved design

Full-screen live terminal monitor for all local Claude Code sessions. Replaces/extends
the iTerm2 toolbelt Session Status panel; runs in a floating hotkey iTerm window.
Go + Bubble Tea/Lipgloss. Rich per-session display; Enter jumps to the session's tab.

## Research questions

RQ1 · tty→iTerm2-session mapping via AppleScript: does `osascript` reliably resolve
      the signal file's `tty` to a session and `select`/`write text` it, incl. from a
      hotkey window? (gap G2)
RQ2 · Subagent counting: how do sidechains/teammate transcripts manifest on disk —
      inline `isSidechain` entries vs separate JSONL files; what marks completion? (G4)
RQ3 · Transcript schema stability: which JSONL fields survive CC version bumps; what
      is the graceful-degradation contract for reducers? (G3)
RQ4 · Cost/context derivation: per-model context limits + token pricing table needed
      to render gauge and $ without a statusline feed. (G6)

## Hypotheses

H1 (RQ1) · iTerm2's AppleScript dictionary exposes per-session `tty` and supports
      `select` + `write text`; one `osascript` round-trip <150ms. Falsified if tty is
      absent from the dictionary or hotkey-window focus steals break selection.
H2 (RQ2) · Subagent starts/stops are countable from Agent tool_use blocks in the main
      transcript alone (sidechain files not required for a live count).
H3 (RQ3) · Reducers keyed on {type, message.model, message.usage, attributionSkill,
      attributionMcpServer, isSidechain, ai-title, mode, permission-mode} parse
      transcripts from ≥2 CC versions without error when unknown lines are skipped.
H4 (RQ4) · usage.cache_read_input_tokens + input+output tokens vs a static per-model
      limit table renders a context gauge within ~5% of the in-app indicator.

## Method (approved design)

Single Go binary, 3 layers; no daemon, no state files.

- **collector**: fsnotify on `~/.cache/claude-tab-status/` (signal JSONs:
  status/tty/pid/cwd/ts) + incremental tail of live transcript JSONLs under
  `~/.claude/projects/<cwd-slug>/<session-id>.jsonl` (byte-offset resume, per-line
  reducers, tolerate unknown lines) + lazy history scanner.
- **model**: per-session struct merging signal + reduced transcript state: status,
  ai-title, project, branch, model+effort, context gauge, token/cost sparkline,
  skills, workflows, MCP servers, subagent count, current tool call, todos,
  permission mode, turn count, duration, CC version.
- **ui (Bubble Tea)**: fullscreen alt-screen, responsive to narrow hotkey window.
  List of rich rows; header aggregates (counts by state, cost today, burn sparkline).
  Keys: `enter` jump · `→`/`tab` detail pane (live tail) · `h` cycle
  live→+recent→history · `k` interrupt (^C) · `p` prompt-compose minibuffer · `q`.
  Closed sessions: jump opens new tab cd'd to cwd with `claude -r <id>` prefilled,
  not sent.
- **iterm bridge**: `osascript` AppleScript, 4 verbs — jump, kill, send-prompt,
  new-tab-at-cwd. Behind an interface; mocked in tests.
- **liveness (G1)**: live = signal fresh (<1800s) ∧ (transcript mtime recent ∨ pid
  alive); recent = exited <30 min (dimmed); history = rest, sorted by mtime.
- **testing**: table tests on reducers with fixture JSONL from real transcripts;
  teatest for UI; mocked bridge; one live smoke script.

Spikes to run first (answer RQ1/RQ2 before UI work): osascript tty probe; sidechain
disk-structure inspection during a live subagent run.

## Evidence so far

- [[research-plan-cowtop-tui]] — shipped Textual monitor; ring-buffer/offset lessons
  carried into collector design (monotonic byte offsets).
- [[learning-textual-tui-testing-gotchas]] — pty testing needs pexpect-style harness;
  Go analogue: teatest.
- Local recon (this session): hook.sh writes per-session signal JSON with
  tty/pid/cwd/status; transcript JSONL verified to carry model, usage (incl. cache +
  thinking tokens), effort, ai-title, attributionSkill, attributionMcpServer,
  isSidechain, gitBranch, slug, permission-mode — all rich fields harvestable (G3).
- Framework probe (awesome-cli-coding-agents): Bubble Tea/Lipgloss = Crush/OpenCode
  stack, chosen; ratatui and Ink rejected (dashboard styling cost / inline-UI bias).

## Done-criteria

D1 · `claude_top` binary launches fullscreen in an iTerm hotkey window and lists all
     live sessions with status, ai-title, project, model within 2s of a hook signal.
D2 · Enter on a live row activates the correct iTerm2 tab (verified across ≥2 windows,
     ≥5 tabs); on a closed row opens a new tab at the session cwd with `claude -r`
     prefilled.
D3 · Detail pane shows context gauge, cost, skills, MCP servers, subagent count for a
     real session, matching manual transcript inspection.
D4 · `h` cycles the three modes; `k` interrupts a running session; `p` sends a prompt
     that arrives in the target session.
D5 · Reducer test suite green on fixture transcripts from ≥2 CC versions; unknown
     lines never crash.
D6 · Coexists with iterm2-tab-status adapter + native toolbelt for a full workday, no
     signal-dir write conflicts.

## Loop-shaped?

Yes — multi-session build with verifier-checkable gates (D1–D6). Suitable for
loop-system INIT; equally buildable via plan-and-execute if preferred.
