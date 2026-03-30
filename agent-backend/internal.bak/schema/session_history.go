package schema

import (
	"agent-backend/pkg/gai/memory"
)

type GetSessionHistory struct {
	SessionID int `json:"session_id"`
	Limit     int `json:"limit"`
	Offset    int `json:"offset"`
}

type SessionHistory struct {
	messages memory.Message `json:""`
}
