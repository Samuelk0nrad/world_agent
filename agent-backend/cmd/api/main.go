package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"agent-backend/agent"
	"agent-backend/config"
)

func main() {
	if err := run(); err != nil {
		fmt.Printf("error running the service: %s", err)
	}
}

func run() error {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	config, err := config.NewEnv(".env", true)
	if err != nil {
		return err
	}

	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	logger := log.New(os.Stdout, "agent-backend: ", log.LstdFlags|log.Lmicroseconds)

	srv := agent.New(config, logger)
	return srv.Start(ctx)
}
