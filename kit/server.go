package kit

import (
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/renniemaharaj/go-api-kit/internal/environment"
)

// Run builds and serves an application using environment-driven defaults.
// The optional frontend root stays open until the server exits.
func Run(build func(fs.FS) (http.Handler, error)) error {
	if build == nil {
		return fmt.Errorf("kit: handler builder is required")
	}

	values, err := environment.Load(".env")
	if err != nil {
		return err
	}

	var fsys fs.FS
	if directory := values.String("FRONTEND_DIR", ""); directory != "" {
		root, err := os.OpenRoot(directory)
		if err != nil {
			return err
		}
		defer root.Close()

		fsys = root.FS()
	}

	handler, err := build(fsys)
	if err != nil {
		return err
	}
	if handler == nil {
		return fmt.Errorf("kit: handler is required")
	}

	server, err := newServer(handler, values)
	if err != nil {
		return err
	}

	return server.ListenAndServe()
}

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
		return nil, fmt.Errorf("kit: SERVER_ADDRESS is required")
	}

	if timeout <= 0 || server.ReadHeaderTimeout <= 0 || server.ReadTimeout <= 0 || server.WriteTimeout <= 0 || server.IdleTimeout <= 0 {
		return nil, fmt.Errorf("kit: request and server timeouts must be positive")
	}

	return server, nil
}
