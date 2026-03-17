package observability

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestMetadataFromRequestDefaults(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	metadata := MetadataFromRequest(req)

	if metadata.RequestID == "" {
		t.Fatal("expected generated request id")
	}
	if metadata.UserID != "unknown" {
		t.Fatalf("expected user placeholder unknown, got %q", metadata.UserID)
	}
	if metadata.DeviceID != "unknown" {
		t.Fatalf("expected device placeholder unknown, got %q", metadata.DeviceID)
	}
}

func TestSetTaskIDUpdatesRequestContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	SetTaskID(ctx, "task-123")

	metadata := MetadataFromGin(ctx)
	if metadata.TaskID != "task-123" {
		t.Fatalf("expected task id task-123, got %q", metadata.TaskID)
	}
	fromRequest := MetadataFromContext(ctx.Request.Context())
	if fromRequest.TaskID != "task-123" {
		t.Fatalf("expected task id task-123 in request context, got %q", fromRequest.TaskID)
	}
}

func TestRequestMetadataMiddlewareSetsHeaderAndContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestMetadataMiddleware(nil))
	router.GET("/check", func(c *gin.Context) {
		metadata := MetadataFromGin(c)
		c.JSON(http.StatusOK, metadata)
	})

	req := httptest.NewRequest(http.MethodGet, "/check", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var metadata Metadata
	if err := json.Unmarshal(recorder.Body.Bytes(), &metadata); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if metadata.RequestID == "" {
		t.Fatal("expected request id in response")
	}
	if metadata.UserID != "unknown" || metadata.DeviceID != "unknown" {
		t.Fatalf("expected unknown placeholders, got user=%q device=%q", metadata.UserID, metadata.DeviceID)
	}
	if recorder.Header().Get("X-Request-Id") != metadata.RequestID {
		t.Fatalf("expected X-Request-Id header %q, got %q", metadata.RequestID, recorder.Header().Get("X-Request-Id"))
	}
}
