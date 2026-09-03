package kit

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// Middleware is standard net/http middleware.
type Middleware = func(http.Handler) http.Handler

// Router is a net/http-compatible router backed internally by chi.
type Router struct{ r chi.Router }

// New creates an empty Router.
func New() *Router { return &Router{r: chi.NewRouter()} }

// ServeHTTP implements http.Handler.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) { r.r.ServeHTTP(w, req) }

// Use appends standard net/http middleware to the router's middleware stack.
func (r *Router) Use(middlewares ...Middleware) { r.r.Use(middlewares...) }

// Route creates an inline sub-router mounted at pattern.
func (r *Router) Route(pattern string, fn func(*Router)) {
	r.r.Route(pattern, func(cr chi.Router) { fn(&Router{r: cr}) })
}

// Group creates an inline router group with an inherited middleware stack.
func (r *Router) Group(fn func(*Router)) {
	r.r.Group(func(cr chi.Router) { fn(&Router{r: cr}) })
}

// Method registers a handler for an HTTP method and route pattern.
func (r *Router) Method(method, pattern string, fn HandlerFunc) {
	r.r.Method(method, pattern, NewHandler(fn))
}

func (r *Router) Get(pattern string, fn HandlerFunc)     { r.Method(http.MethodGet, pattern, fn) }
func (r *Router) Post(pattern string, fn HandlerFunc)    { r.Method(http.MethodPost, pattern, fn) }
func (r *Router) Put(pattern string, fn HandlerFunc)     { r.Method(http.MethodPut, pattern, fn) }
func (r *Router) Patch(pattern string, fn HandlerFunc)   { r.Method(http.MethodPatch, pattern, fn) }
func (r *Router) Delete(pattern string, fn HandlerFunc)  { r.Method(http.MethodDelete, pattern, fn) }
func (r *Router) Head(pattern string, fn HandlerFunc)    { r.Method(http.MethodHead, pattern, fn) }
func (r *Router) Options(pattern string, fn HandlerFunc) { r.Method(http.MethodOptions, pattern, fn) }

// Param returns the route parameter named key, or an empty string if absent.
func (h Handle) Param(key string) string { return chi.URLParam(h.r, key) }
