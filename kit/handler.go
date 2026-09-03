// Package kit provides a small, net/http-compatible handler and router API.
package kit

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// HandlerFunc is a request handler that receives a request-scoped Handle.
type HandlerFunc func(Handle)

// Handle contains the standard HTTP request and response writer together with
// small conveniences for common request and response operations.
type Handle struct {
	w http.ResponseWriter
	r *http.Request

	Error  ErrorAPI
	Header HeaderAPI

	// UnmarshalJSON decodes exactly one JSON value from the request body.
	// It is a function field because Go reserves the method name for
	// json.Unmarshaler's incompatible UnmarshalJSON([]byte) error signature.
	UnmarshalJSON func(v any) error
}

func newHandle(w http.ResponseWriter, r *http.Request) Handle {
	return Handle{
		w:             w,
		r:             r,
		Error:         ErrorAPI{w: w},
		Header:        HeaderAPI{w: w, r: r},
		UnmarshalJSON: func(v any) error { return unmarshalJSON(r, v) },
	}
}

// NewHandler adapts a kit HandlerFunc to an http.HandlerFunc.
func NewHandler(fn HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fn(newHandle(w, r))
	}
}

// Writer returns the underlying response writer.
func (h Handle) Writer() http.ResponseWriter { return h.w }

// Request returns the underlying request.
func (h Handle) Request() *http.Request { return h.r }

// Path returns the URL path for the request.
func (h Handle) Path() string { return h.r.URL.Path }

// Query returns the first value for a URL query parameter.
func (h Handle) Query(name string) string { return h.r.URL.Query().Get(name) }

// QueryValues returns all parsed URL query parameters.
func (h Handle) QueryValues() url.Values { return h.r.URL.Query() }

// Status writes a response status with no body.
func (h Handle) Status(code int) { h.w.WriteHeader(code) }

// NoContent writes a 204 No Content response.
func (h Handle) NoContent() { h.Status(http.StatusNoContent) }

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

// JSON marshals value and writes it as a JSON response. It marshals before
// writing the status, so a marshal error leaves the response untouched.
func (h Handle) JSON(code int, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	h.w.Header().Set("Content-Type", "application/json; charset=utf-8")
	h.w.WriteHeader(code)
	_, err = h.w.Write(append(data, '\n'))
	return err
}

// unmarshalJSON decodes exactly one JSON value from the request body into v.
func unmarshalJSON(r *http.Request, v any) error {
	if r.Body == nil {
		return errors.New("request body is empty")
	}

	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(v); err != nil {
		return err
	}

	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("request body must contain a single JSON value")
		}
		return fmt.Errorf("decode trailing JSON data: %w", err)
	}
	return nil
}

// Redirect sends an HTTP redirect response.
func (h Handle) Redirect(location string, code int) {
	http.Redirect(h.w, h.r, location, code)
}

// Cookie returns the named cookie from the request.
func (h Handle) Cookie(name string) (*http.Cookie, error) { return h.r.Cookie(name) }

// SetCookie adds a Set-Cookie header to the response.
func (h Handle) SetCookie(cookie *http.Cookie) { http.SetCookie(h.w, cookie) }

// ErrorAPI writes conventional plain-text HTTP errors.
type ErrorAPI struct{ w http.ResponseWriter }

// Status writes a plain-text HTTP error with the supplied status.
func (e ErrorAPI) Status(code int, message string) { http.Error(e.w, message, code) }

func (e ErrorAPI) BadRequest(message string)   { e.Status(http.StatusBadRequest, message) }
func (e ErrorAPI) Unauthorized(message string) { e.Status(http.StatusUnauthorized, message) }
func (e ErrorAPI) Forbidden(message string)    { e.Status(http.StatusForbidden, message) }
func (e ErrorAPI) NotFound(message string)     { e.Status(http.StatusNotFound, message) }
func (e ErrorAPI) Internal(message string)     { e.Status(http.StatusInternalServerError, message) }

// HeaderAPI provides request and response header operations.
type HeaderAPI struct {
	w http.ResponseWriter
	r *http.Request
}

// Get returns the first value of a request header.
func (h HeaderAPI) Get(name string) string { return h.r.Header.Get(name) }

// Values returns all values for a request header.
func (h HeaderAPI) Values(name string) []string { return h.r.Header.Values(name) }

// Set sets a response header.
func (h HeaderAPI) Set(name, value string) { h.w.Header().Set(name, value) }

// Add adds a response header value.
func (h HeaderAPI) Add(name, value string) { h.w.Header().Add(name, value) }

// Del removes a response header.
func (h HeaderAPI) Del(name string) { h.w.Header().Del(name) }
