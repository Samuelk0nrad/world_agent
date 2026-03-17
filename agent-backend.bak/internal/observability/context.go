package observability

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type Metadata struct {
	RequestID string
	UserID    string
	DeviceID  string
	TaskID    string
}

type contextKey string

const (
	metadataContextKey contextKey = "observability_metadata"
	ginMetadataKey                = "observability_metadata"
)

func MetadataFromRequest(r *http.Request) Metadata {
	requestID := r.Header.Get("X-Request-Id")
	if requestID == "" {
		requestID = fmt.Sprintf("req-%d", time.Now().UnixNano())
	}

	userID := r.Header.Get("X-User-Id")
	if userID == "" {
		userID = "unknown"
	}

	deviceID := r.Header.Get("X-Device-Id")
	if deviceID == "" {
		deviceID = "unknown"
	}

	return Metadata{
		RequestID: requestID,
		UserID:    userID,
		DeviceID:  deviceID,
		TaskID:    r.Header.Get("X-Task-Id"),
	}
}

func WithMetadata(ctx context.Context, metadata Metadata) context.Context {
	return context.WithValue(ctx, metadataContextKey, metadata)
}

func MetadataFromContext(ctx context.Context) Metadata {
	if metadata, ok := ctx.Value(metadataContextKey).(Metadata); ok {
		return metadata
	}
	return Metadata{}
}

func MetadataFromGin(c *gin.Context) Metadata {
	if value, ok := c.Get(ginMetadataKey); ok {
		if metadata, ok := value.(Metadata); ok {
			return metadata
		}
	}

	metadata := MetadataFromContext(c.Request.Context())
	if metadata.RequestID == "" {
		metadata = MetadataFromRequest(c.Request)
		c.Request = c.Request.WithContext(WithMetadata(c.Request.Context(), metadata))
	}
	c.Set(ginMetadataKey, metadata)
	return metadata
}

func SetTaskID(c *gin.Context, taskID string) {
	if taskID == "" {
		return
	}

	metadata := MetadataFromGin(c)
	metadata.TaskID = taskID
	c.Set(ginMetadataKey, metadata)
	c.Request = c.Request.WithContext(WithMetadata(c.Request.Context(), metadata))
}

func (m Metadata) LogFields() []any {
	fields := []any{
		"request_id", m.RequestID,
		"user_id", m.UserID,
		"device_id", m.DeviceID,
	}
	if m.TaskID != "" {
		fields = append(fields, "task_id", m.TaskID)
	}
	return fields
}
