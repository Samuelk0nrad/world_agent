package main

import (
	"fmt"

	"agent-backend/internal/api/server"
	"agent-backend/internal/config"
)

func main() {
	env := config.NewEnv(".env", true)

	router := server.NewRouter(env)

	err := router.Run(":8080")
	if err != nil {
		fmt.Print("error accured running the api")
	}
}
