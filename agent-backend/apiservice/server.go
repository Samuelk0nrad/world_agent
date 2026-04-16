package apiservice

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"agent-backend/config"
	aicontext "agent-backend/gai/context"
	"agent-backend/store"
)

type AgentServer struct {
	handler      *http.Handler
	config       *config.Env
	logger       *log.Logger
	sessionStore aicontext.SessionStore
}

func New(
	config *config.Env,
	logger *log.Logger,
) (*AgentServer, error) {
	mux := http.NewServeMux()

	sessionStore := store.NewInMemorySessionStore()
	providerRepo, err := RegisterProviders(*config, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to register AI providers: %w", err)
	}
	addRoutes(
		mux,
		config,
		logger,
		sessionStore,
		providerRepo,
	)

	var handler http.Handler = mux
	// middleware
	handler = endpointLogging(logger, handler)
	return &AgentServer{
		handler: &handler,
		config:  config,
		logger:  logger,
	}, nil
}

func (s *AgentServer) Start(ctx context.Context) error {
	srv := &http.Server{
		Addr:    net.JoinHostPort(s.config.Host, s.config.Port),
		Handler: *s.handler,
	}

	errCh := make(chan error, 1)
	go func() {
		s.logger.Printf("starting listening on %s:%s...\n", s.config.Host, s.config.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Printf("server error: %s\n", err)
			errCh <- err
		}
	}()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()

		select {
		case <-ctx.Done():
		case err := <-errCh:
			s.logger.Printf("server error: %s\n", err)
			return
		}

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			s.logger.Printf("server shutdown error: %s\n", err)
		}
	}()

	wg.Wait()
	return nil
}
