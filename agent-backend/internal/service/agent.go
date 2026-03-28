package service

import (
	"context"
	"fmt"
	"sync"

	"agent-backend/internal/config"
	"agent-backend/internal/schema"
	"agent-backend/pkg/gai/ai"
	gemini "agent-backend/pkg/gai/ai_gemini"
	"agent-backend/pkg/gai/loop"
)

type AgentService struct {
	model      ai.Model
	tools      []loop.Tool
	promptPath string
	sessions   map[int]*SessionAgent
	sessionsMu sync.Mutex
}

type SessionAgent struct {
	Agent *loop.Agent
	Mu    sync.Mutex
}

func NewAgentService(env *config.Env) (*AgentService, error) {
	model, err := gemini.New(env.GeminiAPIKey).Model(gemini.Gemini2_5Flash)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize model: %w", err)
	}

	return &AgentService{
		model:      model,
		tools:      []loop.Tool{loop.NewEchoTool()},
		promptPath: env.PromptPath,
		sessions:   make(map[int]*SessionAgent),
	}, nil
}

func (s *AgentService) ProcessAgentRequest(ctx context.Context, req *schema.AgentRequest) (*schema.AgentResponse, error) {
	if req.SessionID <= 0 {
		return nil, fmt.Errorf("sessionId must be a positive integer")
	}

	sessionAgent, err := s.getOrCreateSessionAgent(req.SessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get or create session: %w", err)
	}

	sessionAgent.Mu.Lock()
	defer sessionAgent.Mu.Unlock()

	message, err := sessionAgent.Agent.FollowUp(ctx, req.Prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to process agent request: %w", err)
	}

	messages, err := sessionAgent.Agent.MemorySystem.GetMessages(0)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve messages: %w", err)
	}

	return &schema.AgentResponse{
		Prompt:   req.Prompt,
		Message:  message,
		Messages: messages,
	}, nil
}

func (s *AgentService) getOrCreateSessionAgent(sessionID int) (*SessionAgent, error) {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()

	if existing, ok := s.sessions[sessionID]; ok {
		return existing, nil
	}

	agent, err := loop.NewAgentFromPromptFiles(
		s.model,
		s.tools,
		s.promptPath+"/system.md",
		s.promptPath+"/toolCall.md",
		sessionID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create agent: %w", err)
	}

	created := &SessionAgent{Agent: agent}
	s.sessions[sessionID] = created
	return created, nil
}

func (s *AgentService) Close() error {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()

	if s.model != nil {
		if err := s.model.Close(); err != nil {
			return fmt.Errorf("failed to close model: %w", err)
		}
	}
	s.sessions = map[int]*SessionAgent{}
	return nil
}
