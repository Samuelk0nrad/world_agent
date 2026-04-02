package main

import (
	"fmt"
	"net"
	"net/http"

	"agent-backend/agent"
	"agent-backend/config"
)

func main() {
	if err := run(); err != nil {
		fmt.Printf("error running the service: %s", err)
	}
}

func run() error {
	config, err := config.NewEnv(".env", true)
	if err != nil {
		return err
	}
	srv := agent.NewServer(config)
	httpServer := &http.Server{
		Addr:    net.JoinHostPort(config.Host, config.Port),
		Handler: srv,
	}
	fmt.Printf("starting listening on %s:%s...\n", config.Host, config.Port)
	return httpServer.ListenAndServe()
}
