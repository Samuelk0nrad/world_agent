package agent

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

type ApiResponse[T any] struct {
	Data    *T     `json:"data,omitempty"`
	Message string `json:"message,omitempty"`
}

type ErrWithStatus struct {
	status int
	err    error
}

func (e *ErrWithStatus) Error() string {
	return e.err.Error()
}

func encode[T any](w http.ResponseWriter, r *http.Request, status int, v T) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		return fmt.Errorf("encode json: %w", err)
	}
	return nil
}

func decode[T any](r *http.Request, data *T) (T, error) {
	var v T
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return v, fmt.Errorf("decode json: %w", err)
	}
	return v, nil
}

func handler(f func(w http.ResponseWriter, r *http.Request) error, logger *log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := f(w, r); err != nil {
			status := http.StatusInternalServerError
			msg := http.StatusText(status)

			if e, ok := err.(*ErrWithStatus); ok {
				status = e.status
				msg = http.StatusText(status)
				if status == http.StatusBadRequest {
					msg = e.Error()
				}
			}

			logger.Printf("error handling request: %s\n", err)
			if err := encode(w, r, status, ApiResponse[any]{
				Message: msg,
			}); err != nil {
				logger.Printf("error encoding error response: %s\n", err)
			}
		}
	}
}
