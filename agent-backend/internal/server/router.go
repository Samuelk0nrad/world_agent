package server

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"worldagent/agent-backend/internal/agent"
	"worldagent/agent-backend/internal/connectors"
	"worldagent/agent-backend/internal/extensions"
	"worldagent/agent-backend/internal/llm"
	"worldagent/agent-backend/internal/observability"
	"worldagent/agent-backend/internal/store"

	"github.com/gin-gonic/gin"
)

const (
	logEventsDefaultLimit = 200
	logEventsMaxLimit     = 2000
)

func NewRouter(cfg Config) *gin.Engine {
	logger := observability.NewLogger(cfg.LogLevel, cfg.LogFormat)
	auditSink := observability.AuditSink(observability.NopAuditSink{})
	var auditStore observability.AuditEventStore
	if cfg.LogEventsEnabled {
		memorySink := observability.NewInMemoryAuditSink(cfg.LogEventBuffer)
		auditSink = memorySink
		auditStore = memorySink
	}

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(observability.RequestMetadataMiddleware(logger))

	memoryStore := store.NewFileStore(cfg.MemoryFile)
	registry := extensions.NewDefaultRegistry()

	connectorRegistry := cfg.ConnectorRegistry
	if connectorRegistry == nil {
		connectorRegistry = connectors.NewRegistry()
		var webSearchConnector connectors.WebSearchConnector
		serpAPIConnector, connectorErr := connectors.NewSerpAPIWebSearchConnectorFromEnv()
		if connectorErr != nil {
			webSearchConnector = connectors.NewUnavailableWebSearchConnector(connectorErr)
		} else {
			webSearchConnector = serpAPIConnector
		}
		if err := connectorRegistry.Register(webSearchConnector); err != nil {
			logger.Error("failed to register default web-search connector", "error", err.Error())
		}
	}
	if _, ok := connectorRegistry.Get(connectors.GmailConnectorID); !ok {
		var emailConnector connectors.EmailConnector
		defaultGmailConnector, gmailErr := connectors.NewGmailConnectorFromEnv()
		if gmailErr != nil {
			emailConnector = connectors.NewUnavailableEmailConnector(connectors.GmailConnectorID, gmailErr)
		} else {
			emailConnector = defaultGmailConnector
		}
		if err := connectorRegistry.Register(emailConnector); err != nil {
			logger.Error("failed to register default gmail connector", "error", err.Error())
		}
	}

	responder := cfg.GeminiResponder
	llmConnectorID := strings.ToLower(strings.TrimSpace(cfg.LLMConnector))
	if responder == nil && llmConnectorID != "" && llmConnectorID != "none" {
		switch llmConnectorID {
		case connectors.GeminiConnectorID:
			if _, err := connectors.GetTextGenerationConnector(connectorRegistry, connectors.GeminiConnectorID); err != nil {
				geminiClient, geminiErr := llm.NewGeminiClientFromEnv()
				if geminiErr != nil {
					responder = llm.NewStaticErrorResponder(fmt.Errorf("gemini connector requested but unavailable: %w", geminiErr))
				} else if registerErr := connectorRegistry.Register(geminiClient); registerErr != nil {
					responder = llm.NewStaticErrorResponder(fmt.Errorf("register gemini connector: %w", registerErr))
				}
			}
		default:
			responder = llm.NewStaticErrorResponder(fmt.Errorf("unsupported AGENT_LLM_CONNECTOR value %q", cfg.LLMConnector))
		}
	}

	runtimeOptions := []agent.RuntimeOption{
		agent.WithConnectorRegistry(connectorRegistry),
		agent.WithAuditSink(auditSink),
		agent.WithLogPayloads(cfg.LogIncludePayload),
	}
	if llmConnectorID != "" && llmConnectorID != "none" {
		runtimeOptions = append(runtimeOptions, agent.WithLLMConnectorID(llmConnectorID))
	}
	if responder != nil {
		runtimeOptions = append(runtimeOptions, agent.WithResponder(responder))
	}

	runtime := agent.NewRuntime(memoryStore, registry, runtimeOptions...)

	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	router.GET("/v1/extensions", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"extensions": registry.List()})
	})

	router.PATCH("/v1/extensions/:id", func(c *gin.Context) {
		var payload struct {
			Enabled bool `json:"enabled"`
		}
		if err := c.ShouldBindJSON(&payload); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		ext, err := registry.SetEnabled(c.Param("id"), payload.Enabled)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"extension": ext})
	})

	router.GET("/v1/memory", func(c *gin.Context) {
		sinceParam := c.Query("since")

		var entries []any
		if sinceParam == "" {
			allEntries, err := memoryStore.ListAll()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			entries = make([]any, len(allEntries))
			for i, entry := range allEntries {
				entries[i] = entry
			}
		} else {
			since, err := strconv.ParseInt(sinceParam, 10, 64)
			if err != nil || since < 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "since must be a non-negative integer"})
				return
			}

			sinceEntries, err := memoryStore.ListSince(since)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			entries = make([]any, len(sinceEntries))
			for i, entry := range sinceEntries {
				entries[i] = entry
			}
		}

		latestSequence, err := memoryStore.LatestSequence()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"entries":         entries,
			"latest_sequence": latestSequence,
		})
	})

	router.POST("/v1/memory", func(c *gin.Context) {
		var payload struct {
			Source  string `json:"source"`
			Content string `json:"content"`
		}
		if err := c.ShouldBindJSON(&payload); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if payload.Content == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "content is required"})
			return
		}
		if payload.Source == "" {
			payload.Source = "mobile"
		}

		entry, err := memoryStore.Append(payload.Source, payload.Content)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"entry": entry})
	})

	router.POST("/v1/agent/run", func(c *gin.Context) {
		// Preserve the existing API request/response shape while the runtime now
		// executes via the evented loop core under the hood.
		var payload struct {
			Message           string `json:"message"`
			MaxSteps          int    `json:"maxSteps"`
			GoogleAccessToken string `json:"googleAccessToken"`
		}
		if err := c.ShouldBindJSON(&payload); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		accessToken := strings.TrimSpace(payload.GoogleAccessToken)
		if accessToken == "" {
			accessToken = strings.TrimSpace(c.GetHeader("X-Google-Access-Token"))
		}

		taskID := fmt.Sprintf("task-%d", time.Now().UnixNano())
		observability.SetTaskID(c, taskID)
		metadata := map[string]string{
			"max_steps":        strconv.Itoa(payload.MaxSteps),
			"has_google_token": strconv.FormatBool(accessToken != ""),
		}
		if cfg.LogIncludePayload {
			metadata["message"] = payload.Message
		} else {
			metadata["message_chars"] = strconv.Itoa(len(strings.TrimSpace(payload.Message)))
		}
		_ = auditSink.Record(c.Request.Context(), observability.AuditEvent{
			Type:     observability.EventAgentRunRequested,
			Message:  "Agent run requested",
			Metadata: metadata,
		})

		ctx := connectors.WithEmailAccessToken(c.Request.Context(), accessToken)
		result, err := runtime.RunWithContext(ctx, payload.Message, payload.MaxSteps)
		if err != nil {
			_ = auditSink.Record(c.Request.Context(), observability.AuditEvent{
				Type:    observability.EventToolFailed,
				Tool:    "agent-run",
				Message: "Agent run failed",
				Error:   err.Error(),
			})
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		_ = auditSink.Record(c.Request.Context(), observability.AuditEvent{
			Type:    observability.EventAgentRunCompleted,
			Message: "Agent run completed",
		})
		c.JSON(http.StatusOK, gin.H{"result": result})
	})

	if cfg.LogAPIEnabled && auditStore != nil {
		router.GET("/v1/logs/events", func(c *gin.Context) {
			since, err := parseLogEventsSince(c.Query("since"))
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}

			limit, err := parseLogEventsLimit(c.Query("limit"))
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}

			eventType := observability.EventType(strings.ToLower(strings.TrimSpace(c.Query("type"))))
			events := auditStore.EventsSince(since, limit, eventType)
			sort.Slice(events, func(i, j int) bool {
				return events[i].Sequence < events[j].Sequence
			})
			if events == nil {
				events = make([]observability.AuditEvent, 0)
			}
			c.JSON(http.StatusOK, gin.H{
				"events":          events,
				"latest_sequence": auditStore.LatestSequence(),
			})
		})
	}

	return router
}

func parseLogEventsSince(raw string) (int64, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, nil
	}

	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed < 0 {
		return 0, fmt.Errorf("since must be a non-negative integer")
	}
	return parsed, nil
}

func parseLogEventsLimit(raw string) (int, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return logEventsDefaultLimit, nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("limit must be a positive integer")
	}
	if parsed > logEventsMaxLimit {
		return logEventsMaxLimit, nil
	}
	return parsed, nil
}
