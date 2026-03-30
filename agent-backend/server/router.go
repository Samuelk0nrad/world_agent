package server

import (
	"fmt"
	"net"
	"net/http"

	"agent-backend/config"
)

func handler(http.ResponseWriter, *http.Request) {
	fmt.Printf("Hello World")
}

func NewRouter(env *config.Env) {
	http.HandleFunc("GET /hello", handler)

	fmt.Println("Server listening on port 8080 ....")
	if err := http.ListenAndServe(
		net.JoinHostPort(env.Host, env.Port),
		nil,
	); err != nil {
		fmt.Printf("error listening on port 8080: %s", err.Error())
	}
}
