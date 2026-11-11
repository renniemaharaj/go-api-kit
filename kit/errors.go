package kit

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"slices"
)

// ErrorRule maps a domain error, including wrapped errors, to a public response.
// Rules match in declaration order and never expose the original error text.
type ErrorRule struct {
	Error   error
	Status  int
	Message string
}

// ErrorConfig defines known error responses and the fallback message for HTTP 500.
// Empty messages use the standard HTTP status text.
type ErrorConfig struct {
	Rules           []ErrorRule
	InternalMessage string
}

// ErrorAPI writes conventional plain-text HTTP errors.
type ErrorAPI struct {
	w      http.ResponseWriter
	config ErrorConfig
}

// WithErrors binds a snapshot of the error configuration to each request handle.
func (fn HandlerFunc) WithErrors(config ErrorConfig) (HandlerFunc, error) {
	if fn == nil {
		return nil, fmt.Errorf("kit: handler is required")
	}

	for _, rule := range config.Rules {
		if rule.Error == nil {
			return nil, fmt.Errorf("kit: error rule requires an error")
		}

		if rule.Status < 400 || rule.Status > 599 {
			return nil, fmt.Errorf("kit: error rule status must be between 400 and 599")
		}
	}

	config.Rules = slices.Clone(config.Rules)

	return func(h Handle) {
		h.Error.config = config
		fn(h)
	}, nil
}

// Handle translates an error and logs server failures without exposing details.
// A nil error leaves the response untouched.
func (e ErrorAPI) Handle(err error) {
	if err == nil {
		return
	}

	status, message := e.response(err)
	if status >= http.StatusInternalServerError {
		log.Printf("request failed: %v", err)
	}

	e.Status(status, message)
}

func (e ErrorAPI) response(err error) (int, string) {
	for _, rule := range e.config.Rules {
		if !errors.Is(err, rule.Error) {
			continue
		}

		message := rule.Message
		if message == "" {
			message = http.StatusText(rule.Status)
		}

		return rule.Status, message
	}

	message := e.config.InternalMessage
	if message == "" {
		message = http.StatusText(http.StatusInternalServerError)
	}

	return http.StatusInternalServerError, message
}

// Status writes a plain-text HTTP error with the supplied status.
func (e ErrorAPI) Status(code int, message string) {
	http.Error(e.w, message, code)
}

func (e ErrorAPI) BadRequest(message string) {
	e.Status(http.StatusBadRequest, message)
}

func (e ErrorAPI) Unauthorized(message string) {
	e.Status(http.StatusUnauthorized, message)
}

func (e ErrorAPI) Forbidden(message string) {
	e.Status(http.StatusForbidden, message)
}

func (e ErrorAPI) NotFound(message string) {
	e.Status(http.StatusNotFound, message)
}

func (e ErrorAPI) Internal(message string) {
	e.Status(http.StatusInternalServerError, message)
}
