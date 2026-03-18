package loop

import "errors"

var (
	ErrNilAgent           = errors.New("agent is nil")
	ErrModelNotConfigured = errors.New("model is not configured")
	ErrEmptyPrompt        = errors.New("prompt is empty")
)
