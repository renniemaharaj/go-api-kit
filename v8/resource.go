// Package v8 assembles resource applications with a built-in health endpoint.
package v8

import (
	"fmt"
	"io/fs"
	"net/http"
	"slices"
	"time"

	"github.com/renniemaharaj/go-api-kit/data"
	"github.com/renniemaharaj/go-api-kit/kit"
)

type Snapshot struct {
	StartedAt time.Time
}

type Status struct {
	Status        string `json:"status"`
	UptimeSeconds int64  `json:"uptime_seconds"`
}

// Run starts the application using kit's environment-driven server defaults.
func Run(resources ...data.Definition) error {
	return kit.Run(func(fsys fs.FS) (http.Handler, error) {
		return NewHandler(fsys, resources...)
	})
}

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
	resources = append(resources, newResource())
	if fsys != nil {
		frontend, err := newFrontend(fsys)
		if err != nil {
			return nil, err
		}

		resources = append(resources, frontend)
	}

	return data.NewEngine(kit.New(middleware...), resources...)
}

func newResource() data.Definition {
	storage := newRepository()
	checks := &service{repository: storage}
	handlers := handler{service: checks}

	return data.Definition{
		Name:   "health",
		Path:   "/health",
		Routes: handlers.Routes(),
	}
}

func newFrontend(fsys fs.FS) (data.Definition, error) {
	files, err := kit.NewFileServerFromEnv(fsys)
	if err != nil {
		return data.Definition{}, err
	}

	serve := func(request kit.Handle) {
		files.ServeHTTP(request.Writer(), request.Request())
	}

	return data.Definition{
		Name: "frontend",
		Routes: []data.Route{
			{
				Method:  http.MethodGet,
				Path:    "/*",
				Handler: serve,
			},
			{
				Method:  http.MethodHead,
				Path:    "/*",
				Handler: serve,
			},
		},
	}, nil
}
