# claude_top workflow visibility — research plan

## Research questions

RQ1 · Can claude_top detect that a session's Workflow tool invocation is
currently running, without hooking Workflow internals, using only what's
already on disk in the session transcript + its subagent dirs?

RQ2 · What's the ground-truth signal for "workflow started" / "workflow
ended", as opposed to a time-window liveness proxy (the pattern the
project ledger already flags as risky — PROJECT-LEARNINGS.md §4)?

RQ3 · What progress unit is honestly derivable given no per-agent phase
attribution survives to disk (G2)?

RQ4 · Why does claude_top show nothing today for a running workflow?

## Hypotheses

H1 (RQ1) · The main session transcript's `tool_use name="Workflow"` line
carries a paired `toolUseResult` with `taskType:"local_workflow"`,
`taskId`, `workflowName`, `transcriptDir`, `summary` — sufficient to
detect a launch without touching the workflow script or its subagents.

H2 (RQ2) · A later `<task-notification>` line in the SAME main transcript,
keyed by the same `taskId`, with `status` completed/killed, is the
ground-truth end event — the launch/notification pair fully brackets
"running," with no mtime window required for the workflow-level flag.

H3 (RQ3) · Per-agent phase labels are not recoverable cheaply (no `phase`
field in `meta.json` or `journal.jsonl`), but agent counts (live/total,
same mtime-window method already used for Task-tool subagents) are —
"N/M agents" is the honest unit, not a phase name.

H4 (RQ4) · `internal/subagents/subagents.go` `Count()` never recurses into
`subagents/workflows/wf_*/`, because it does a non-recursive `ReadDir`
that explicitly skips directories — this alone explains total silence.

## Method

RQ1/H1 — read a live, currently-running workflow's transcript directly
(not a synthetic fixture) and confirm the `toolUseResult` schema fields
by inspection. Done.

RQ2/H2 — read a COMPLETED workflow's transcript from a different project
to confirm the notification schema and that its `taskId` matches the
launch event's `taskId`. Done.

RQ3/H3 — read `journal.jsonl` and a `.meta.json` from the live workflow's
`transcriptDir`; enumerate every distinct `type`/field seen. Done.

RQ4/H4 — read `internal/subagents/subagents.go` and trace call sites in
`internal/registry/registry.go` and `internal/ui/view.go`. Done.

Implementation method (post-plan): extend `transcript.line`/`reduceLine`
to parse the launch + notification events into new `Stats` fields
(`WorkflowRunning`, `WorkflowName`, `WorkflowSummary`,
`WorkflowTranscriptDir`), tracked via a private `Reader` field for the
pending `taskId` (mirrors existing `lastUsageMsgID` dedup pattern).
Extract `subagents.Count()`'s directory-scan body into a `CountDir`
helper reusable against a workflow's `transcriptDir`. Wire both into
`registry.EnsureStats` and a new `view.go` render line, gated on
`WorkflowRunning`.

## Evidence so far

- Live sample: pj_rufus session `e81d27c5-480a-4173-9150-75884197a1b8`,
  workflow `wf_4d1b5e16-14b` (`rufus-nonparcel-merge-court`, taskId
  `wv27w70gp`) — confirmed `toolUseResult` schema: `{status:
  "async_launched", taskId, taskType:"local_workflow", workflowName,
  runId, summary, transcriptDir, scriptPath}`.
- Same workflow's `transcriptDir`: 12 `agent-<hash>.jsonl` +
  `.meta.json` pairs (`{agentType:"workflow-subagent", spawnDepth,
  model}` — no phase field) + one `journal.jsonl` (20 lines, only
  `type ∈ {started, result}`, keyed by `agentId` + content-hash `key` —
  no phase field either).
- `scriptPath` (readable `.js`) carries `meta.phases:[{title}...]` and
  `phase('X')` calls in source, but nothing correlates a specific agent
  run to a specific phase on disk.
- Completed-workflow sample: pj_rufus session `29bc9fa6-…` — a
  `queue-operation`/bare `user`-type line carrying
  `<task-notification><task-id>{taskId}</task-id>…<status>completed
  </status>…<summary>Dynamic workflow "…"` appears in the MAIN
  transcript on finish, keyed by the same `taskId` from the launch
  event. Same mechanism also fires for backgrounded Bash/Agent tasks
  (`<summary>Background command "…"`), so it's a general
  backgrounded-task notification, not workflow-specific — this feature
  only consumes the `local_workflow`-tagged instance of it.
- `internal/subagents/subagents.go:26-34` — `Count()` does
  `os.ReadDir(sessionDir/subagents)`, explicitly `if e.IsDir() {
  continue }` — confirmed root cause of RQ4/H4: a workflow's
  `subagents/workflows/wf_*/` subtree is structurally unreachable.
- `internal/transcript/transcript.go` already increments a lifetime
  `Stats.Workflows` counter on `tool_use name=="Workflow"`
  (line 235-236) but drops `toolUseResult` and task-notification content
  entirely today — the hook point already half-exists.
- Project ledger rule directly on point (PROJECT-LEARNINGS.md §4):
  "Derive a state from ground truth, not a time window standing in for
  it" — motivates H2 over reusing `subagentLiveWindow` for the
  workflow-level flag; per-agent counts inside a workflow still use the
  window heuristic (same shape as today's Task-tool subagent display,
  not the flagged anti-pattern).
- Design approved 2026-09-14 (brainstorming DIVERGE): show workflow name
  + summary + live/total agent counts; ground-truth event pair for the
  running flag; current run only, no history.

## Done-criteria

1. A session actively running a Workflow tool invocation renders a
   distinct line in claude_top's TUI showing the workflow name, its
   one-line summary, and `{live}/{total}` agent counts — verified live
   against a real running workflow (not just a synthetic fixture).
2. The line disappears (or the flag flips false) within one refresh
   cycle after the matching `<task-notification>` with status
   completed/killed appears in the transcript — verified against a real
   completed workflow's transcript replayed through the reducer.
3. `go test ./...` passes, including new cases in `transcript_test.go`
   (launch→running→notification→done transition, using synthetic lines
   built from the confirmed real schema) and `subagents_test.go`
   (`CountDir` against a `workflows/wf_x/`-shaped fixture, `time.Now()`-
   relative mtimes per the project's fixture-honesty rule).
4. No change to existing subagent/Task-tool display behavior or its
   tests (`internal/subagents/subagents.go`'s public `Count()` signature
   and behavior unchanged).

## Loop-shaped?

No — single bounded code change, one session, done-criteria are
directly verifier-checkable (tests + one live transcript replay), no
multi-session iteration or open-ended goal.
