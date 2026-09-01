# RQ1 · osascript iTerm2 bridge probe — PASS (all verbs)

Run live 2026-09-01 by orchestrator (classifier blocked delegating terminal-driving
to a subagent; see STATE Open failures). No macOS automation-consent prompt fired
(consent pre-granted on this machine).

| Verb | Working syntax | Timing |
|---|---|---|
| enumerate | `repeat with w in windows / tabs of w / sessions of t` reading `id of w`, manual tab counter (NB: `index of t` in repeat-with fails, error -1700 — keep own counter), `id of s`, `tty of s`, `name of s` | 0.63s for 9 sessions / 3 windows |
| select by tty | scan as above; on match `select t` then `select s` | included in 1.5s round trip |
| jump/new tab | `tell <window> to set newTab to (create tab with default profile)`; `current session of newTab`; restore via saved `current tab of current window` + `select` | — |
| write unsubmitted | `write s text "..." newline NO` — verified lands in input without executing (`contents of s` shows it) | — |
| write submitted | `write s text "..."` | — |
| interrupt (^C) | `write s text (string id 3) newline NO` — verified `^C` kills `sleep 999` | — |
| read contents | `contents of s` | — |

Full round trip (create scratch tab → refocus origin → find+select by tty → write
newline NO → read contents → close tab → restore focus): **1.5s incl. 0.6s of
deliberate delays**. Output: `scratchTty=/dev/ttys003 | foundById=A2157B4F-… |
contentsHasProbe=true`; interrupt probe: `interrupted=true`.

Design consequences:
- H1 CONFIRMED. Bridge = plain `osascript` exec from Go; no Python API needed.
- Per-call latency dominated by our own `delay` calls; bare calls are ~100–600ms.
- Enumeration is cheap enough to run on demand (on Enter), no tty→session cache
  needed for v1.
- `claude -r <id>` prefill for closed sessions: create tab, `write text "cd <cwd>"`
  (submitted), then `write text "claude -r <id>" newline NO`.
