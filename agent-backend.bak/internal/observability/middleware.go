package observability

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

func RequestMetadataMiddleware(logger *slog.Logger) gin.HandlerFunc {
	if logger == nil {
		logger = slog.Default()
	}

	return func(c *gin.Context) {
		start := time.Now()
		metadata := MetadataFromRequest(c.Request)
		c.Set(ginMetadataKey, metadata)
		c.Request = c.Request.WithContext(WithMetadata(c.Request.Context(), metadata))
		c.Writer.Header().Set("X-Request-Id", metadata.RequestID)

		c.Next()

		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}

		fields := append(metadata.LogFields(),
			"method", c.Request.Method,
			"path", path,
			"status", c.Writer.Status(),
			"latency_ms", time.Since(start).Milliseconds(),
		)
		if len(c.Errors) > 0 {
			fields = append(fields, "errors", c.Errors.String())
			logger.Error("request completed with errors", fields...)
			return
		}
		logger.Info("request completed", fields...)
	}
}
