package apiservice

import (
	"context"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"agent-backend/config"
)

type AgentServer struct {
	handler *http.Handler
	config  *config.Env
	logger  *log.Logger
}

func New(
	config *config.Env,
	logger *log.Logger,
) *AgentServer {
	mux := http.NewServeMux()
	addRoutes(mux,
		config,
		logger,
	)
	// middleware
	var handler http.Handler = mux
	handler = endpointLogging(logger, handler)
	return &AgentServer{
		handler: &handler,
		config:  config,
		logger:  logger,
	}
}

func (s *AgentServer) Start(ctx context.Context) error {
	srv := &http.Server{
		Addr:    net.JoinHostPort(s.config.Host, s.config.Port),
		Handler: *s.handler,
	}

	go func() {
		s.logger.Printf("starting listening on %s:%s...\n", s.config.Host, s.config.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Printf("server error: %s\n", err)
		}
	}()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-ctx.Done()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			s.logger.Printf("server shutdown error: %s\n", err)
		}
	}()

	wg.Wait()
	return nil
}
