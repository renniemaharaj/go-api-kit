package kit

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

// Middleware is standard net/http middleware.
type Middleware = func(http.Handler) http.Handler

// Router is a net/http-compatible router backed internally by chi.
type Router struct {
	r      chi.Router
	prefix string
	routes *routeRegistry
}

// New installs environment-driven defaults unless a replacement stack is supplied.
// Invalid default settings panic at construction; Use appends extra middleware.
func New(middlewares ...Middleware) *Router {
	if middlewares == nil {
		var err error
		middlewares, err = DefaultMiddleware()
		if err != nil {
			panic(err)
		}
	}

	validateMiddleware(middlewares)

	router := &Router{
		r:      chi.NewRouter(),
		routes: &routeRegistry{probe: chi.NewRouter()},
	}
	router.r.Use(middlewares...)

	return router
}

// ServeHTTP implements http.Handler.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.r.ServeHTTP(w, req)
}

// Use appends middleware and returns the same router for chaining.
func (r *Router) Use(middlewares ...Middleware) *Router {
	validateMiddleware(middlewares)
	r.r.Use(middlewares...)

	return r
}

// With applies middleware to selected endpoints after route parameters are matched.
func (r *Router) With(middlewares ...Middleware) *Router {
	validateMiddleware(middlewares)

	return &Router{r: r.r.With(middlewares...), prefix: r.prefix, routes: r.routes}
}

// Route creates an inline sub-router mounted at pattern.
func (r *Router) Route(pattern string, fn func(*Router)) {
	r.r.Route(pattern, func(cr chi.Router) {
		fn(&Router{r: cr, prefix: strings.TrimSuffix(r.prefix, "/") + pattern, routes: r.routes})
	})
}

// Group creates an inline router group with an inherited middleware stack.
func (r *Router) Group(fn func(*Router)) {
	r.r.Group(func(cr chi.Router) {
		fn(&Router{r: cr, prefix: r.prefix, routes: r.routes})
	})
}

// Method registers a handler for an HTTP method and route pattern.
func (r *Router) Method(method, pattern string, fn HandlerFunc) {
	if fn == nil {
		panic("kit: handler is required")
	}

	r.Handle(method, pattern, fn)
}

// Handle registers a standard net/http handler for a method and pattern.
// Invalid or conflicting routes panic during setup, before serving requests.
func (r *Router) Handle(method, pattern string, handler http.Handler) {
	if handler == nil {
		panic("kit: handler is required")
	}

	if !strings.HasPrefix(pattern, "/") {
		panic("kit: route pattern must start with /")
	}

	method = strings.ToUpper(method)
	r.routes.add(method, strings.TrimSuffix(r.prefix, "/")+pattern)
	r.r.Method(method, pattern, handler)
}

func (r *Router) Get(pattern string, fn HandlerFunc) {
	r.Method(http.MethodGet, pattern, fn)
}

func (r *Router) Post(pattern string, fn HandlerFunc) {
	r.Method(http.MethodPost, pattern, fn)
}

func (r *Router) Put(pattern string, fn HandlerFunc) {
	r.Method(http.MethodPut, pattern, fn)
}

func (r *Router) Patch(pattern string, fn HandlerFunc) {
	r.Method(http.MethodPatch, pattern, fn)
}

func (r *Router) Delete(pattern string, fn HandlerFunc) {
	r.Method(http.MethodDelete, pattern, fn)
}

func (r *Router) Head(pattern string, fn HandlerFunc) {
	r.Method(http.MethodHead, pattern, fn)
}

func (r *Router) Options(pattern string, fn HandlerFunc) {
	r.Method(http.MethodOptions, pattern, fn)
}

// Param returns the route parameter named key, or an empty string if absent.
func (h Handle) Param(key string) string {
	return chi.URLParam(h.r, key)
}

func validateMiddleware(middleware []Middleware) {
	for _, wrap := range middleware {
		if wrap == nil {
			panic("kit: middleware cannot be nil")
		}
	}
}

// Probe the same routing syntax without running user middleware or handlers.
type routeRegistry struct {
	probe chi.Router
	count int
}

func (r *routeRegistry) add(method, pattern string) {
	r.probe.Method(method, pattern, http.NotFoundHandler())
	count := 0
	err := chi.Walk(r.probe, func(string, string, http.Handler, ...func(http.Handler) http.Handler) error {
		count++
		return nil
	})
	if err != nil {
		panic(err)
	}

	if count != r.count+1 {
		panic(fmt.Sprintf("kit: conflicting route %s %s", method, pattern))
	}

	r.count = count
}
