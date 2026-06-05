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
