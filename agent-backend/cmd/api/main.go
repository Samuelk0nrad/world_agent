package main

import (
	"fmt"
)

func main() {
	if err := run(); err != nil {
		fmt.Printf("error running the service: %s", err)
	}
}

func run() error {
	return nil
}
