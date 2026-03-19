package loop

import "errors"

var (
	ErrNilAgent           = errors.New("agent is nil")
	ErrModelNotConfigured = errors.New("model is not configured")
	ErrEmptyPrompt        = errors.New("prompt is empty")
	ErrNilResponseBuilder = errors.New("response builder is nil")
	ErrInvalidToolRequest = errors.New("invalid tool request")
	ErrToolNotFound       = errors.New("tool not found")
	ErrMaxIterations      = errors.New("max loop iterations exceeded")
	ErrPromptPathEmpty    = errors.New("prompt path is empty")
	ErrPromptFileType     = errors.New("prompt file must be .md or .txt")
)
