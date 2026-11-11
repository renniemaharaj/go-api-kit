package data

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/renniemaharaj/go-api-kit/kit"
)

// Engine wires resource definitions into a router and serves the application.
// Construct it before serving requests; resource definitions are read once.
type Engine struct {
	router *kit.Router
}

// NewEngine validates all definitions before registering any resource routes.
// The supplied router owns application-wide middleware and existing routes.
func NewEngine(router *kit.Router, definitions ...Definition) (*Engine, error) {
	if router == nil {
		return nil, fmt.Errorf("data: router is required")
	}

	routes, err := prepareRoutes(definitions)
	if err != nil {
		return nil, err
	}

	if err := validateRoutes(routes); err != nil {
		return nil, err
	}

	for _, route := range routes {
		router.With(route.middleware...).Method(route.method, route.path, route.handler)
	}

	return &Engine{router: router}, nil
}

func (e *Engine) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	e.router.ServeHTTP(w, r)
}

type resourceRoute struct {
	resource   string
	method     string
	path       string
	handler    kit.HandlerFunc
	middleware []kit.Middleware
}

func prepareRoutes(definitions []Definition) ([]resourceRoute, error) {
	var routes []resourceRoute
	for _, definition := range definitions {
		if strings.TrimSpace(definition.Name) == "" {
			return nil, fmt.Errorf("data: resource name is required")
		}

		if definition.Path != "" && !strings.HasPrefix(definition.Path, "/") {
			return nil, fmt.Errorf("data: %s: resource path must start with /", definition.Name)
		}

		if err := validateMiddleware(definition.Middleware); err != nil {
			return nil, fmt.Errorf("data: %s: %w", definition.Name, err)
		}

		for _, route := range definition.Routes {
			if route.Path != "" && !strings.HasPrefix(route.Path, "/") {
				return nil, fmt.Errorf("data: %s: route path must start with /", definition.Name)
			}

			if route.Handler == nil {
				return nil, fmt.Errorf("data: %s: route handler is required", definition.Name)
			}

			if err := validateMiddleware(route.Middleware); err != nil {
				return nil, fmt.Errorf("data: %s: %w", definition.Name, err)
			}

			handler, err := route.Handler.WithErrors(definition.Errors)
			if err != nil {
				return nil, fmt.Errorf("data: %s: %w", definition.Name, err)
			}

			path := strings.TrimSuffix(definition.Path, "/") + route.Path
			if path == "" {
				path = "/"
			}

			middleware := append([]kit.Middleware(nil), definition.Middleware...)
			middleware = append(middleware, route.Middleware...)
			routes = append(routes, resourceRoute{
				resource:   definition.Name,
				method:     strings.ToUpper(route.Method),
				path:       path,
				handler:    handler,
				middleware: middleware,
			})
		}
	}

	return routes, nil
}

func validateMiddleware(middleware []kit.Middleware) error {
	for _, wrap := range middleware {
		if wrap == nil {
			return fmt.Errorf("middleware cannot be nil")
		}
	}

	return nil
}
