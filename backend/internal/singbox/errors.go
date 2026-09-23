package singbox

import "errors"

var (
	ErrNotInstalled   = errors.New("sing-box not installed")
	ErrNoConfig       = errors.New("no config")
	ErrAlreadyRunning = errors.New("already running")
	ErrNotRunning     = errors.New("not running")
	ErrInvalidJSON    = errors.New("invalid json")
)

// InvalidConfigError carries the `sing-box check` output for a bad config.
type InvalidConfigError struct{ Output string }

func (e *InvalidConfigError) Error() string { return "invalid config" }

// StartFailedError means sing-box passed `check` but exited immediately after
// being spawned — most often a listen port already held by another process.
// Output is the tail of its log, which is the only place the reason appears.
type StartFailedError struct{ Output string }

func (e *StartFailedError) Error() string { return "sing-box exited immediately after start" }
