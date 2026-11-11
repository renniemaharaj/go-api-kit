package kit_test

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"testing"

	"github.com/renniemaharaj/go-api-kit/kit"
)

func TestHandleErrorMapsDomainErrorsAndKeepsServerDetailsPrivate(t *testing.T) {
	notFound := errors.New("missing record")
	invalid := errors.New("invalid record")
	unexpected := errors.New("private storage failure")

	var logged bytes.Buffer
	previousOutput := log.Writer()
	log.SetOutput(&logged)
	t.Cleanup(func() {
		log.SetOutput(previousOutput)
	})

	config := kit.ErrorConfig{
		Rules: []kit.ErrorRule{
			{
				Error:   notFound,
				Status:  http.StatusNotFound,
				Message: "user not found",
			},
			{
				Error:   invalid,
				Status:  http.StatusBadRequest,
				Message: "user ID and name are required",
			},
		},
		InternalMessage: "could not complete user operation",
	}

	for _, test := range []struct {
		name   string
		err    error
		status int
		body   string
		logged bool
	}{
		{
			name:   "wrapped missing record",
			err:    fmt.Errorf("users: get: %w", notFound),
			status: http.StatusNotFound,
			body:   "user not found\n",
		},
		{
			name:   "validation",
			err:    fmt.Errorf("users: save: %w", invalid),
			status: http.StatusBadRequest,
			body:   "user ID and name are required\n",
		},
		{
			name:   "first matching rule",
			err:    errors.Join(invalid, notFound),
			status: http.StatusNotFound,
			body:   "user not found\n",
		},
		{
			name:   "unexpected failure",
			err:    unexpected,
			status: http.StatusInternalServerError,
			body:   "could not complete user operation\n",
			logged: true,
		},
		{
			name:   "nil leaves response untouched",
			status: http.StatusNoContent,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			logged.Reset()
			handler, err := kit.HandlerFunc(func(h kit.Handle) {
				h.Error.Handle(test.err)
				if test.err == nil {
					h.NoContent()
				}
			}).WithErrors(config)
			if err != nil {
				t.Fatal(err)
			}

			response := request(t, kit.HandlerFunc(handler), http.MethodGet, "/", "")
			if response.Code != test.status || response.Body.String() != test.body {
				t.Fatalf("got %d %q, want %d %q", response.Code, response.Body.String(), test.status, test.body)
			}

			if test.logged && !strings.Contains(logged.String(), unexpected.Error()) {
				t.Fatal("server failure was not logged")
			}

			if !test.logged && logged.Len() != 0 {
				t.Fatalf("expected error was logged: %s", logged.String())
			}
		})
	}
}

func TestErrorConfigurationIsCopiedAndUsesDefaultMessages(t *testing.T) {
	missing := errors.New("missing")
	config := kit.ErrorConfig{
		Rules: []kit.ErrorRule{
			{
				Error:  missing,
				Status: http.StatusNotFound,
			},
		},
	}
	handler, err := kit.HandlerFunc(func(h kit.Handle) {
		h.Error.Handle(missing)
	}).WithErrors(config)
	if err != nil {
		t.Fatal(err)
	}

	config.Rules[0].Status = http.StatusForbidden
	config.Rules[0].Message = "changed after construction"

	response := request(t, kit.HandlerFunc(handler), http.MethodGet, "/", "")
	if response.Code != http.StatusNotFound || response.Body.String() != "Not Found\n" {
		t.Fatalf("error configuration changed after binding: %d %q", response.Code, response.Body.String())
	}

	unconfigured := kit.HandlerFunc(func(h kit.Handle) {
		h.Error.Handle(errors.New("private details"))
	})
	response = request(t, unconfigured, http.MethodGet, "/", "")
	if response.Code != http.StatusInternalServerError || response.Body.String() != "Internal Server Error\n" {
		t.Fatalf("unexpected default response: %d %q", response.Code, response.Body.String())
	}
}

func TestHandlerRejectsInvalidErrorRules(t *testing.T) {
	for _, rule := range []kit.ErrorRule{
		{
			Status: http.StatusNotFound,
		},
		{
			Error:  errors.New("invalid status"),
			Status: http.StatusOK,
		},
		{
			Error:  errors.New("invalid status"),
			Status: 600,
		},
	} {
		_, err := kit.HandlerFunc(func(h kit.Handle) {
			h.NoContent()
		}).WithErrors(kit.ErrorConfig{Rules: []kit.ErrorRule{rule}})
		if err == nil {
			t.Fatalf("accepted invalid error rule: %#v", rule)
		}
	}

	var handler kit.HandlerFunc
	if _, err := handler.WithErrors(kit.ErrorConfig{}); err == nil {
		t.Fatal("accepted a nil handler")
	}
}
