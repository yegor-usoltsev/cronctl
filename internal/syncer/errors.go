package syncer

import "errors"

var (
	errBuildNeedsRoot    = errors.New("build requires root to switch user")
	errUserEmpty         = errors.New("user is empty")
	errScheduleCronEmpty = errors.New("cron is empty")
	errInvalidEnvKey     = errors.New("invalid env key")
	errNegativeID        = errors.New("negative")
	errIDTooLarge        = errors.New("too large")
	errPathTraversal     = errors.New("path traversal detected")
)
