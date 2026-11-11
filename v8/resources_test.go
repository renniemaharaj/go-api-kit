package v8_test

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/renniemaharaj/go-api-kit/data"
	"github.com/renniemaharaj/go-api-kit/examples/resources/users"
	"github.com/renniemaharaj/go-api-kit/kit"
	"github.com/renniemaharaj/go-api-kit/v8"
)

func isolateEnvironment(t *testing.T) {
	t.Helper()
	t.Chdir(t.TempDir())

	for _, name := range []string{
		"RATE_LIMIT_REQUESTS",
		"RATE_LIMIT_INTERVAL",
		"RATE_LIMIT_BURST",
		"RATE_LIMIT_MAX_CLIENTS",
		"RATE_LIMIT_IDLE_TTL",
		"JWT_ENABLED",
		"JWT_SECRET",
		"JWT_PUBLIC_KEY",
		"JWT_REQUIRED_ROLES",
		"CORS_ALLOWED_ORIGINS",
		"REQUEST_TIMEOUT",
		"FRONTEND_DIR",
	} {
		t.Setenv(name, "")
		if err := os.Unsetenv(name); err != nil {
			t.Fatal(err)
		}
	}
}

func request(t *testing.T, app http.Handler, method, path, body, token string, status int) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}

	response := httptest.NewRecorder()
	app.ServeHTTP(response, request)
	if response.Code != status {
		t.Fatalf("%s %s: got %d %q, want %d", method, path, response.Code, response.Body.String(), status)
	}

	return response
}

func TestResourcesUseServicesAndRepositoriesAlongsideFrontend(t *testing.T) {
	isolateEnvironment(t)
	resource, err := users.New(users.User{
		ID:   "42",
		Name: "Ada",
	})
	if err != nil {
		t.Fatal(err)
	}

	authenticate := kit.Authenticate(func(_ context.Context, request *http.Request) (kit.Principal, error) {
		if request.Header.Get("Authorization") != "Bearer allowed" {
			return kit.Principal{}, kit.ErrUnauthenticated
		}

		return kit.Principal{ID: "42"}, nil
	})
	frontend := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("frontend")},
		"users/42":   &fstest.MapFile{Data: []byte("must not shadow the resource")},
	}
	app, err := v8.NewHandler(frontend,
		resource.Definition(authenticate, kit.RequireSubject("id")),
	)
	if err != nil {
		t.Fatal(err)
	}

	page := request(t, app, http.MethodGet, "/", "", "", http.StatusOK)
	if page.Body.String() != "frontend" {
		t.Fatalf("unexpected frontend response: %q", page.Body.String())
	}

	response := request(t, app, http.MethodGet, "/users/42", "", "allowed", http.StatusOK)
	var user users.User
	if err := json.Unmarshal(response.Body.Bytes(), &user); err != nil {
		t.Fatal(err)
	}

	if user.Name != "Ada" {
		t.Fatalf("resource did not load repository data: %#v", user)
	}

	request(t, app, http.MethodPut, "/users/42", `{"name":"Grace"}`, "allowed", http.StatusOK)
	request(t, app, http.MethodPut, "/users/42", `{"name":" "}`, "allowed", http.StatusBadRequest)

	user, err = resource.Get(context.Background(), "42")
	if err != nil || user.Name != "Grace" {
		t.Fatalf("service validation or cache invalidation failed: %#v, %v", user, err)
	}

	request(t, app, http.MethodGet, "/users/42", "", "", http.StatusUnauthorized)
	request(t, app, http.MethodGet, "/users/57", "", "allowed", http.StatusForbidden)
	request(t, app, http.MethodDelete, "/users/42", "", "allowed", http.StatusNoContent)

	missing := request(t, app, http.MethodGet, "/users/42", "", "allowed", http.StatusNotFound)
	if missing.Body.String() != "user not found\n" {
		t.Fatalf("resource error mapping was lost: %q", missing.Body.String())
	}
}

func TestEnvLimiterIsSharedByFilesResourcesAndMissingRoutes(t *testing.T) {
	isolateEnvironment(t)
	settings := "RATE_LIMIT_REQUESTS=1\nRATE_LIMIT_INTERVAL=1h\nRATE_LIMIT_BURST=3\nRATE_LIMIT_IDLE_TTL=3h\n"
	if err := os.WriteFile(".env", []byte(settings), 0o600); err != nil {
		t.Fatal(err)
	}

	resource, err := users.New(users.User{
		ID:   "42",
		Name: "Ada",
	})
	if err != nil {
		t.Fatal(err)
	}

	frontend := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("frontend")},
	}
	app, err := v8.NewHandler(frontend, resource.Definition())
	if err != nil {
		t.Fatal(err)
	}

	request(t, app, http.MethodGet, "/", "", "", http.StatusOK)
	request(t, app, http.MethodGet, "/users/42", "", "", http.StatusOK)
	request(t, app, http.MethodGet, "/missing", "", "", http.StatusNotFound)
	request(t, app, http.MethodGet, "/users/42?view=full", "", "", http.StatusTooManyRequests)
}

func TestV8ReturnsConfigurationAndDefinitionErrors(t *testing.T) {
	isolateEnvironment(t)
	resource, err := users.New()
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(".env", []byte("RATE_LIMIT_BURST=invalid\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := v8.NewHandler(nil, resource.Definition()); err == nil {
		t.Fatal("invalid environment settings were accepted")
	}

	stack := []kit.Middleware{kit.Timeout(time.Second)}
	if _, err := v8.NewHandlerWithMiddleware(nil, stack, resource.Definition()); err != nil {
		t.Fatalf("explicit middleware must replace the default stack: %v", err)
	}

	if _, err := v8.NewHandler(nil); err == nil {
		t.Fatal("an application without files or resources was accepted")
	}

	conflict := data.Definition{
		Name: "conflict",
		Routes: []data.Route{
			{
				Method: http.MethodGet,
				Path:   "/*",
				Handler: func(h kit.Handle) {
					h.NoContent()
				},
			},
		},
	}
	if _, err := v8.NewHandlerWithMiddleware(fstest.MapFS{}, stack, conflict); err == nil {
		t.Fatal("a resource conflicting with the frontend route was accepted")
	}
}

func TestRunSupportsAnAPIWithoutAFrontendDirectory(t *testing.T) {
	isolateEnvironment(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	t.Setenv("SERVER_ADDRESS", listener.Addr().String())

	resource, err := users.New()
	if err != nil {
		t.Fatal(err)
	}

	err = v8.Run(resource.Definition())
	var listenError *net.OpError
	if !errors.As(err, &listenError) || listenError.Op != "listen" {
		t.Fatalf("API-only startup did not reach the listener: %v", err)
	}
}
