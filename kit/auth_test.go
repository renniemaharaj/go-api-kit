package kit_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/renniemaharaj/go-api-kit/data"
	"github.com/renniemaharaj/go-api-kit/kit"
)

func TestAuthorizationRunsBeforeCachedReads(t *testing.T) {
	loads := 0
	users, err := data.NewCache(func(context.Context, string) (string, error) {
		loads++

		return "private data", nil
	}, data.CacheConfig[string]{TTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}

	verified := 0
	router := kit.New()
	router.Use(
		kit.Authenticate(func(ctx context.Context, r *http.Request) (kit.Principal, error) {
			verified++
			switch r.Header.Get("Authorization") {
			case "Bearer editor":
				return kit.Principal{
					ID:    "editor",
					Roles: []string{"editor"},
				}, nil
			case "Bearer reader":
				return kit.Principal{
					ID:    "reader",
					Roles: []string{"reader"},
				}, nil
			default:
				return kit.Principal{}, kit.ErrUnauthenticated
			}
		}),
		kit.RequireAnyRole("editor"),
	)
	router.Get("/private", func(h kit.Handle) {
		value, err := users.Get(h.Context(), "42")
		if err != nil {
			h.Error.Internal("load failed")
			return
		}

		h.Text(http.StatusOK, value)
	})

	for _, test := range []struct {
		credential string
		status     int
	}{
		{
			credential: "Bearer editor",
			status:     http.StatusOK,
		},
		{
			credential: "Bearer editor",
			status:     http.StatusOK,
		},
		{
			credential: "Bearer reader",
			status:     http.StatusForbidden,
		},
		{
			credential: "",
			status:     http.StatusUnauthorized,
		},
	} {
		request := httptest.NewRequest(http.MethodGet, "/private", nil)
		request.Header.Set("Authorization", test.credential)

		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != test.status {
			t.Fatalf("%q: got %d, want %d", test.credential, response.Code, test.status)
		}

		if test.status != http.StatusOK && response.Body.String() == "private data" {
			t.Fatal("unauthorized caller received cached private data")
		}
	}

	if loads != 1 || verified != 4 {
		t.Fatalf("loads=%d verified=%d", loads, verified)
	}
}

func TestAuthenticationFailsClosed(t *testing.T) {
	for _, test := range []struct {
		name      string
		principal kit.Principal
		err       error
		status    int
	}{
		{
			name:   "missing identity",
			status: http.StatusUnauthorized,
		},
		{
			name:   "invalid credential",
			err:    kit.ErrUnauthenticated,
			status: http.StatusUnauthorized,
		},
		{
			name:   "provider failure",
			err:    errors.New("private provider details"),
			status: http.StatusInternalServerError,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			handler := kit.Authenticate(func(context.Context, *http.Request) (kit.Principal, error) {
				return test.principal, test.err
			})(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				t.Error("authentication failure reached the handler")
			}))

			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
			if response.Code != test.status || response.Body.String() == "private provider details\n" {
				t.Fatalf("got %d %q", response.Code, response.Body.String())
			}
		})
	}
}

func TestAuthorizationRequiresIdentityAndPreservesRoles(t *testing.T) {
	roles := []string{"editor"}
	handler := kit.Authenticate(func(context.Context, *http.Request) (kit.Principal, error) {
		return kit.Principal{
			ID:    "42",
			Roles: roles,
		}, nil
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := kit.PrincipalFromContext(r.Context())
		if !ok {
			t.Error("principal missing")
		}

		principal.Roles[0] = "changed"
		principal, _ = kit.PrincipalFromContext(r.Context())
		if principal.Roles[0] != "editor" || roles[0] != "editor" {
			t.Error("principal exposed mutable roles")
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	protected := kit.RequireAnyRole("editor")(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("missing authentication reached handler")
	}))
	response := httptest.NewRecorder()
	protected.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("missing identity returned %d", response.Code)
	}
}
