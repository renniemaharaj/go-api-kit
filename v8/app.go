// Package v8 starts HTTP applications with defaults and a built-in health endpoint.
package v8

import (
	"fmt"
	"io/fs"
	"net/http"

	"github.com/renniemaharaj/go-api-kit/kit"
)

// Run starts an application. Each optional setup function registers routes directly.
func Run(setup ...func(*kit.Router)) error {
	return kit.Run(func(fsys fs.FS) (http.Handler, error) {
		return NewHandler(fsys, setup...)
	})
}

// NewHandler combines /health, optional routes, and optional frontend files.
// A nil filesystem creates an API-only application, including when setup is empty.
func NewHandler(fsys fs.FS, setup ...func(*kit.Router)) (http.Handler, error) {
	return NewHandlerWithMiddleware(fsys, nil, setup...)
}

// NewHandlerWithMiddleware replaces defaults with a non-nil middleware slice.
// An explicit empty slice disables defaults. Setup errors return a nil handler.
func NewHandlerWithMiddleware(fsys fs.FS, middleware []kit.Middleware, setup ...func(*kit.Router)) (app http.Handler, err error) {
	// Router registration follows net/http's panic-on-invalid-setup convention.
	defer func() {
		if problem := recover(); problem != nil {
			app = nil
			err = fmt.Errorf("v8: route setup: %v", problem)
		}
	}()

	if middleware == nil {
		middleware, err = kit.DefaultMiddleware()
		if err != nil {
			return nil, err
		}
	}

	router := kit.New(middleware...)
	for _, register := range setup {
		if register == nil {
			return nil, fmt.Errorf("v8: route setup function is required")
		}

		register(router)
	}

	health := healthHandler()
	router.Get("/health", health)
	router.Head("/health", health)

	if fsys != nil {
		files, err := kit.NewFileServerFromEnv(fsys)
		if err != nil {
			return nil, err
		}

		router.Handle(http.MethodGet, "/*", files)
		router.Handle(http.MethodHead, "/*", files)
	}

	return router, nil
}
