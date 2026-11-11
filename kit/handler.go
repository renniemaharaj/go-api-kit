// Package kit provides HTTP routing, request middleware, and handler utilities.
package kit

import (
	"context"
	"io"
	"net/http"
	"net/url"
)

// HandlerFunc is an http.Handler that receives a request-scoped Handle.
type HandlerFunc func(Handle)

// Handle contains the standard HTTP request and response writer together with
// small conveniences for common request and response operations.
type Handle struct {
	w http.ResponseWriter
	r *http.Request

	Error ErrorAPI
}

// ServeHTTP implements http.Handler directly, without an adapter closure.
func (fn HandlerFunc) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fn(Handle{
		w:     w,
		r:     r,
		Error: ErrorAPI{w: w},
	})
}

// Writer returns the underlying response writer.
func (h Handle) Writer() http.ResponseWriter {
	return h.w
}

// Request returns the underlying request.
func (h Handle) Request() *http.Request {
	return h.r
}

// Context returns the request context, including its deadline and identity.
func (h Handle) Context() context.Context {
	return h.r.Context()
}

// Path returns the URL path for the request.
func (h Handle) Path() string {
	return h.r.URL.Path
}

// Query returns the first value for a URL query parameter.
func (h Handle) Query(name string) string {
	return h.r.URL.Query().Get(name)
}

// QueryValues returns all parsed URL query parameters.
func (h Handle) QueryValues() url.Values {
	return h.r.URL.Query()
}

// Status writes a response status with no body.
func (h Handle) Status(code int) {
	h.w.WriteHeader(code)
}

// NoContent writes a 204 No Content response.
func (h Handle) NoContent() {
	h.Status(http.StatusNoContent)
}

// Text writes a UTF-8 plain-text response.
func (h Handle) Text(code int, text string) {
	h.w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	h.w.WriteHeader(code)

	_, _ = io.WriteString(h.w, text)
}

// HTML writes a UTF-8 HTML response.
func (h Handle) HTML(code int, html string) {
	h.w.Header().Set("Content-Type", "text/html; charset=utf-8")
	h.w.WriteHeader(code)

	_, _ = io.WriteString(h.w, html)
}

// Redirect sends an HTTP redirect response.
func (h Handle) Redirect(location string, code int) {
	http.Redirect(h.w, h.r, location, code)
}

// Cookie returns the named cookie from the request.
func (h Handle) Cookie(name string) (*http.Cookie, error) {
	return h.r.Cookie(name)
}

// SetCookie adds a Set-Cookie header to the response.
func (h Handle) SetCookie(cookie *http.Cookie) {
	http.SetCookie(h.w, cookie)
}
