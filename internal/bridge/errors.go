package bridge

import "errors"

// ErrBackendUnavailable marks a backend whose program or server is absent
// (tmux not installed, no tmux server running).
var ErrBackendUnavailable = errors.New("terminal backend unavailable")

// ErrUnsupported marks a verb the owning backend cannot perform.
var ErrUnsupported = errors.New("unsupported")
