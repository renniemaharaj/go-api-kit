package data

import "github.com/renniemaharaj/go-api-kit/kit"

// Definition describes HTTP routes, shared middleware, and error responses.
// Path prefixes every route; middleware runs after matching parameters.
type Definition struct {
	Name       string
	Path       string
	Middleware []kit.Middleware
	Routes     []Route
	Errors     kit.ErrorConfig
}

// Route keeps HTTP metadata beside its handler implementation.
// Its middleware runs after the resource's middleware, in declaration order.
type Route struct {
	Method     string
	Path       string
	Handler    kit.HandlerFunc
	Middleware []kit.Middleware
}
