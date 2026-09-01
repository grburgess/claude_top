#!/usr/bin/env bash
# RQ1 probe — iTerm2 AppleScript bridge verbs for claude_top. Safe to re-run:
# creates+closes its own scratch tabs, restores focus. See findings-iterm-probe.md.
set -euo pipefail

echo "== enumerate =="
osascript -e '
tell application "iTerm2"
  set out to ""
  repeat with w in windows
    set ti to 0
    repeat with t in tabs of w
      set ti to ti + 1
      repeat with s in sessions of t
        set out to out & (id of w) & " | tab " & ti & " | " & (id of s) & " | " & (tty of s) & " | " & (name of s) & linefeed
      end repeat
    end repeat
  end repeat
  return out
end tell'

echo "== round trip: scratch tab / select-by-tty / write newline NO / read / cleanup =="
osascript -e '
tell application "iTerm2"
  set origWin to current window
  set origTab to current tab of origWin
  tell origWin to set scratchTab to (create tab with default profile)
  set scratchS to current session of scratchTab
  set scratchTty to tty of scratchS
  delay 0.3
  select origTab
  set found to ""
  repeat with w in windows
    repeat with t in tabs of w
      repeat with s in sessions of t
        if (tty of s) is scratchTty then
          select t
          select s
          set found to (id of s)
        end if
      end repeat
    end repeat
  end repeat
  write scratchS text "echo claude_top_probe" newline NO
  delay 0.3
  set c to contents of scratchS
  close scratchTab
  select origTab
  return "scratchTty=" & scratchTty & " | foundById=" & found & " | contentsHasProbe=" & (c contains "echo claude_top_probe")
end tell'

echo "== interrupt (^C) =="
osascript -e '
tell application "iTerm2"
  set origTab to current tab of current window
  tell current window to set scratchTab to (create tab with default profile)
  set s to current session of scratchTab
  delay 0.5
  write s text "sleep 999"
  delay 0.7
  write s text (string id 3) newline NO
  delay 0.5
  set c to contents of s
  close scratchTab
  select origTab
  return "interrupted=" & (c contains "^C")
end tell'
