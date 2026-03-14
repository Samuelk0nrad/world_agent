package server

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"worldagent/agent-backend/internal/agent"
	"worldagent/agent-backend/internal/connectors"
	"worldagent/agent-backend/internal/extensions"
	"worldagent/agent-backend/internal/llm"
	"worldagent/agent-backend/internal/store"
)

func NewRouter(cfg Config) *gin.Engine {
	router := gin.Default()

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
			log.Printf("failed to register default web-search connector: %v", err)
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
			log.Printf("failed to register default gmail connector: %v", err)
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

	runtimeOptions := []agent.RuntimeOption{agent.WithConnectorRegistry(connectorRegistry)}
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

		ctx := connectors.WithEmailAccessToken(c.Request.Context(), accessToken)
		result, err := runtime.RunWithContext(ctx, payload.Message, payload.MaxSteps)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"result": result})
	})

	return router
}
