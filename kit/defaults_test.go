package kit_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/renniemaharaj/go-api-kit/kit"
)

func isolateDefaultEnvironment(t *testing.T) {
	t.Helper()
	t.Chdir(t.TempDir())

	for _, assignment := range os.Environ() {
		name, _, _ := strings.Cut(assignment, "=")
		for _, prefix := range []string{"RATE_LIMIT_", "REQUEST_", "JWT_", "CORS_", "FILE_CACHE_"} {
			if strings.HasPrefix(name, prefix) {
				t.Setenv(name, "")
				if err := os.Unsetenv(name); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
}

func TestRouterDefaultsLoadPoliciesFromEnvironment(t *testing.T) {
	isolateDefaultEnvironment(t)
	t.Setenv("JWT_SECRET", "default-stack-secret-with-at-least-32-bytes")
	t.Setenv("JWT_REQUIRED_ROLES", "editor")
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://app.example")
	t.Setenv("REQUEST_TIMEOUT", "1s")
	t.Setenv("RATE_LIMIT_REQUESTS", "1")
	t.Setenv("RATE_LIMIT_INTERVAL", "1h")
	t.Setenv("RATE_LIMIT_BURST", "5")
	t.Setenv("RATE_LIMIT_IDLE_TTL", "5h")

	config, err := kit.JWTConfigFromEnv(".env")
	if err != nil {
		t.Fatal(err)
	}

	token := func(id, role string) string {
		t.Helper()

		claims := kit.JWTClaims{
			RegisteredClaims: jwt.RegisteredClaims{
				Issuer:    config.Issuer,
				Audience:  jwt.ClaimStrings{config.Audience},
				Subject:   id,
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			},
			Roles:         []string{role},
			ConfigVersion: config.ConfigVersion,
		}
		signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(config.Key)
		if err != nil {
			t.Fatal(err)
		}

		return signed
	}

	visited := 0
	router := kit.New()
	router.With(kit.RequireSubject("id")).Get("/users/{id}", func(h kit.Handle) {
		visited++
		deadline, exists := h.Context().Deadline()
		if !exists || time.Until(deadline) > time.Second {
			t.Error("default request deadline was not installed")
		}

		h.Text(http.StatusOK, h.Param("id"))
	})

	preflight := httptest.NewRequest(http.MethodOptions, "/users/42", nil)
	preflight.Header.Set("Origin", "https://app.example")
	preflight.Header.Set("Access-Control-Request-Method", "GET")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, preflight)
	if response.Code != http.StatusNoContent {
		t.Fatalf("default CORS preflight required credentials: %d", response.Code)
	}

	for _, test := range []struct {
		token  string
		path   string
		status int
	}{
		{
			path:   "/users/42",
			status: http.StatusUnauthorized,
		},
		{
			token:  token("57", "editor"),
			path:   "/users/42",
			status: http.StatusForbidden,
		},
		{
			token:  token("42", "reader"),
			path:   "/users/42",
			status: http.StatusForbidden,
		},
		{
			token:  token("42", "editor"),
			path:   "/users/42",
			status: http.StatusOK,
		},
		{
			path:   "/missing",
			status: http.StatusTooManyRequests,
		},
	} {
		request := httptest.NewRequest(http.MethodGet, test.path, nil)
		if test.token != "" {
			request.Header.Set("Authorization", "Bearer "+test.token)
		}

		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != test.status {
			t.Fatalf("%s: got %d %q, want %d", test.path, response.Code, response.Body.String(), test.status)
		}
	}

	if visited != 1 {
		t.Fatalf("default policies allowed %d handler calls, want 1", visited)
	}
}

func TestRouterCustomStackReplacesDefaults(t *testing.T) {
	isolateDefaultEnvironment(t)
	t.Setenv("REQUEST_TIMEOUT", "invalid")
	t.Setenv("JWT_ENABLED", "true")

	custom := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Custom", "yes")
			next.ServeHTTP(w, r)
		})
	}
	router := kit.New(custom)
	router.Get("/", func(h kit.Handle) {
		if _, exists := h.Context().Deadline(); exists {
			t.Error("custom stack inherited the default timeout")
		}

		h.NoContent()
	})

	response := request(t, router, http.MethodGet, "/", "")
	if response.Code != http.StatusNoContent || response.Header().Get("X-Custom") != "yes" {
		t.Fatalf("custom stack was not used: %d %v", response.Code, response.Header())
	}
}

func TestDefaultMiddlewareRejectsInvalidEnvironment(t *testing.T) {
	for _, test := range []struct {
		name   string
		values map[string]string
	}{
		{
			name:   "invalid timeout",
			values: map[string]string{"REQUEST_TIMEOUT": "soon"},
		},
		{
			name:   "nonpositive timeout",
			values: map[string]string{"REQUEST_TIMEOUT": "0s"},
		},
		{
			name:   "invalid auth flag",
			values: map[string]string{"JWT_ENABLED": "maybe"},
		},
		{
			name:   "missing signing key",
			values: map[string]string{"JWT_ENABLED": "true"},
		},
		{
			name: "invalid credentialed CORS",
			values: map[string]string{
				"CORS_ALLOWED_ORIGINS":   "*",
				"CORS_ALLOW_CREDENTIALS": "true",
			},
		},
		{
			name:   "roles without authentication",
			values: map[string]string{"JWT_REQUIRED_ROLES": "editor"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			isolateDefaultEnvironment(t)
			for name, value := range test.values {
				t.Setenv(name, value)
			}

			if stack, err := kit.DefaultMiddleware(); err == nil {
				t.Fatalf("invalid settings produced a middleware stack: %v", fmt.Sprint(stack))
			}
		})
	}
}
