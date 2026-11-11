package v8_test

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"testing"
	"testing/fstest"

	"github.com/renniemaharaj/go-api-kit/data"
	"github.com/renniemaharaj/go-api-kit/kit"
	"github.com/renniemaharaj/go-api-kit/v8"
)

func TestHealthIsBuiltInAlongsideResourcesAndFrontend(t *testing.T) {
	isolateEnvironment(t)
	resource := data.Definition{
		Name: "ping",
		Path: "/ping",
		Routes: []data.Route{
			{
				Method: http.MethodGet,
				Handler: func(h kit.Handle) {
					h.NoContent()
				},
			},
		},
	}
	frontend := fstest.MapFS{
		"health": &fstest.MapFile{Data: []byte("must not shadow health")},
	}
	for _, withFrontend := range []bool{true, false} {
		var files fs.FS
		if withFrontend {
			files = frontend
		}

		app, err := v8.NewHandler(files, resource)
		if err != nil {
			t.Fatal(err)
		}

		response := request(t, app, http.MethodGet, "/health", "", "", http.StatusOK)
		var status struct {
			Status        string `json:"status"`
			UptimeSeconds *int64 `json:"uptime_seconds"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &status); err != nil {
			t.Fatal(err)
		}
		if status.Status != "ok" || status.UptimeSeconds == nil || *status.UptimeSeconds < 0 {
			t.Fatalf("unexpected health response: %s", response.Body.String())
		}
		if response.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("health responses must not be cached")
		}

		head := request(t, app, http.MethodHead, "/health", "", "", http.StatusOK)
		if head.Body.Len() != 0 {
			t.Fatal("HEAD returned a body")
		}
		request(t, app, http.MethodPost, "/health", "", "", http.StatusMethodNotAllowed)
		request(t, app, http.MethodGet, "/ping", "", "", http.StatusNoContent)
	}
}

func TestHealthUsesSharedRateLimiter(t *testing.T) {
	isolateEnvironment(t)
	t.Setenv("RATE_LIMIT_REQUESTS", "1")
	t.Setenv("RATE_LIMIT_INTERVAL", "1h")
	t.Setenv("RATE_LIMIT_BURST", "2")
	t.Setenv("RATE_LIMIT_IDLE_TTL", "3h")
	app, err := v8.NewHandler(fstest.MapFS{})
	if err != nil {
		t.Fatal(err)
	}

	request(t, app, http.MethodGet, "/health", "", "", http.StatusOK)
	request(t, app, http.MethodHead, "/health", "", "", http.StatusOK)
	request(t, app, http.MethodGet, "/missing", "", "", http.StatusTooManyRequests)
}

func TestHealthUsesSharedAuthentication(t *testing.T) {
	isolateEnvironment(t)
	t.Setenv("JWT_ENABLED", "true")
	t.Setenv("JWT_SECRET", "health-test-secret-with-at-least-32-bytes")
	app, err := v8.NewHandler(fstest.MapFS{})
	if err != nil {
		t.Fatal(err)
	}

	request(t, app, http.MethodGet, "/health", "", "", http.StatusUnauthorized)
}

func TestHealthRouteConflictsReturnStartupErrors(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		conflict := data.Definition{
			Name: "conflict",
			Path: "/health",
			Routes: []data.Route{
				{
					Method: method,
					Handler: func(h kit.Handle) {
						h.NoContent()
					},
				},
			},
		}

		if _, err := v8.NewHandlerWithMiddleware(nil, []kit.Middleware{}, conflict); err == nil {
			t.Fatalf("conflicting %s /health route was accepted", method)
		}
	}
}
