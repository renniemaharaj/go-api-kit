package v8

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/renniemaharaj/go-api-kit/internal/environment"
)

func newServer(handler http.Handler, values *environment.Reader) (*http.Server, error) {
	timeout := values.Duration("REQUEST_TIMEOUT", 15*time.Second)
	server := &http.Server{
		Addr:              values.String("SERVER_ADDRESS", "localhost:9100"),
		Handler:           handler,
		ReadHeaderTimeout: values.Duration("SERVER_READ_HEADER_TIMEOUT", 5*time.Second),
		ReadTimeout:       values.Duration("SERVER_READ_TIMEOUT", timeout+5*time.Second),
		WriteTimeout:      values.Duration("SERVER_WRITE_TIMEOUT", timeout+5*time.Second),
		IdleTimeout:       values.Duration("SERVER_IDLE_TIMEOUT", time.Minute),
	}
	if err := values.Err(); err != nil {
		return nil, err
	}

	if strings.TrimSpace(server.Addr) == "" {
		return nil, fmt.Errorf("v8: SERVER_ADDRESS is required")
	}

	if timeout <= 0 || server.ReadHeaderTimeout <= 0 || server.ReadTimeout <= 0 || server.WriteTimeout <= 0 || server.IdleTimeout <= 0 {
		return nil, fmt.Errorf("v8: request and server timeouts must be positive")
	}

	return server, nil
}
