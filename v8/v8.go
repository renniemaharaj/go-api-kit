// Package v8 serves environment-configured resource applications and frontend files.
package v8

import (
	"fmt"
	"io/fs"
	"net/http"
	"slices"

	"github.com/renniemaharaj/go-api-kit/data"
	"github.com/renniemaharaj/go-api-kit/internal/environment"
	"github.com/renniemaharaj/go-api-kit/internal/filesystem"
	"github.com/renniemaharaj/go-api-kit/internal/health"
	"github.com/renniemaharaj/go-api-kit/kit"
)

// NewHandler wires resources, /health, and optional files with default middleware.
func NewHandler(fsys fs.FS, resources ...data.Definition) (http.Handler, error) {
	return NewHandlerWithMiddleware(fsys, nil, resources...)
}

// NewHandlerWithMiddleware replaces the default stack when middleware is non-nil.
// An explicit empty slice disables shared middleware; resource policies still run.
func NewHandlerWithMiddleware(fsys fs.FS, middleware []kit.Middleware, resources ...data.Definition) (http.Handler, error) {
	if fsys == nil && len(resources) == 0 {
		return nil, fmt.Errorf("v8: a file system or resource definition is required")
	}

	if middleware == nil {
		var err error
		middleware, err = kit.DefaultMiddleware()
		if err != nil {
			return nil, err
		}
	}

	for _, wrap := range middleware {
		if wrap == nil {
			return nil, fmt.Errorf("v8: middleware cannot be nil")
		}
	}

	resources = slices.Clone(resources)
	resources = append(resources, health.New())
	if fsys != nil {
		frontend, err := frontendDefinition(fsys)
		if err != nil {
			return nil, err
		}

		resources = append(resources, frontend)
	}

	return data.NewEngine(kit.New(middleware...), resources...)
}

// Run reads server settings from the environment and starts the application.
// FRONTEND_DIR is optional when resource definitions are supplied.
func Run(resources ...data.Definition) error {
	values, err := environment.Load(".env")
	if err != nil {
		return err
	}

	var fsys fs.FS
	if directory := values.String("FRONTEND_DIR", ""); directory != "" {
		root, err := filesystem.OpenRoot(directory)
		if err != nil {
			return err
		}
		defer root.Close()

		fsys = root
	}

	handler, err := NewHandler(fsys, resources...)
	if err != nil {
		return err
	}

	server, err := newServer(handler, values)
	if err != nil {
		return err
	}

	return server.ListenAndServe()
}
