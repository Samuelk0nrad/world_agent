package main

import (
	"log"

	"worldagent/agent-backend/internal/server"
)

func main() {
	cfg := server.LoadConfig()
	router := server.NewRouter(cfg)

	log.Printf("agent backend listening on :%s", cfg.Port)
	if err := router.Run(":" + cfg.Port); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
