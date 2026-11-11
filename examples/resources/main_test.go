package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/renniemaharaj/go-api-kit/examples/resources/users"
	"github.com/renniemaharaj/go-api-kit/kit"
	"github.com/renniemaharaj/go-api-kit/v8"
)

type apiTest struct {
	t      *testing.T
	router http.Handler
	token  string
	users  *users.Users
}

func newAPITest(t *testing.T, records ...users.User) *apiTest {
	t.Helper()
	t.Chdir(t.TempDir())
	t.Setenv("RATE_LIMIT_REQUESTS", "20")
	t.Setenv("RATE_LIMIT_INTERVAL", "1s")
	t.Setenv("RATE_LIMIT_BURST", "40")
	t.Setenv("RATE_LIMIT_MAX_CLIENTS", "4096")
	t.Setenv("RATE_LIMIT_IDLE_TTL", "5m")

	resource, err := users.New(records...)
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("JWT_SECRET", "example-test-secret-with-at-least-32-bytes")
	t.Setenv("JWT_ENABLED", "true")
	t.Setenv("JWT_ISSUER", "go-api-kit-example")
	t.Setenv("JWT_AUDIENCE", "users-api")
	t.Setenv("JWT_CONFIG_VERSION", "v1")
	t.Setenv("JWT_REQUIRED_ROLES", "editor")
	t.Setenv("REQUEST_TIMEOUT", "5s")
	router, err := v8.NewHandler(nil,
		func(router *kit.Router) {
			resource.Routes(router, kit.RequireSubject("id"))
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	token, err := issueToken("42")
	if err != nil {
		t.Fatal(err)
	}

	return &apiTest{
		t:      t,
		router: router,
		token:  token,
		users:  resource,
	}
}

func (api *apiTest) request(method, path, body string, status int) *httptest.ResponseRecorder {
	api.t.Helper()

	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+api.token)

	response := httptest.NewRecorder()
	api.router.ServeHTTP(response, request)
	if response.Code != status {
		api.t.Fatalf("%s %s: got %d %s, want %d", method, path, response.Code, response.Body.String(), status)
	}

	return response
}

func (api *apiTest) assertUserName(id, name string) {
	api.t.Helper()

	response := api.request(http.MethodGet, "/users/"+id, "", http.StatusOK)

	var user users.User
	if err := json.Unmarshal(response.Body.Bytes(), &user); err != nil {
		api.t.Fatal(err)
	}

	if user.ID != id || user.Name != name {
		api.t.Fatalf("got %#v, want user %q with name %q", user, id, name)
	}
}

func TestUserAPIWithCachingAndInvalidation(t *testing.T) {
	api := newAPITest(t, users.User{
		ID:   "42",
		Name: "Ada",
	})

	api.assertUserName("42", "Ada")
	api.request(http.MethodPut, "/users/42", `{"id":"57","name":"Wrong user"}`, http.StatusBadRequest)
	api.request(http.MethodPut, "/users/42", `{"name":"Grace"}`, http.StatusOK)
	api.assertUserName("42", "Grace")
	api.request(http.MethodPut, "/users/42", `{"name":" "}`, http.StatusBadRequest)
	api.assertUserName("42", "Grace")
	api.request(http.MethodPut, "/users/42", `{`, http.StatusBadRequest)

	api.request(http.MethodDelete, "/users/42", "", http.StatusNoContent)
	api.request(http.MethodGet, "/users/42", "", http.StatusNotFound)
	api.request(http.MethodDelete, "/users/42", "", http.StatusNotFound)
}

func TestUserAPIBindsJWTToRequestedUser(t *testing.T) {
	api := newAPITest(t,
		users.User{
			ID:   "42",
			Name: "Ada",
		},
		users.User{
			ID:   "57",
			Name: "Grace",
		},
	)

	for _, method := range []string{
		http.MethodGet,
		http.MethodPut,
		http.MethodDelete,
	} {
		api.request(method, "/users/57", `{"name":"Hacked"}`, http.StatusForbidden)
	}

	user, err := api.users.Get(context.Background(), "57")
	if err != nil {
		t.Fatal(err)
	}

	if user.Name != "Grace" {
		t.Fatal("another user's record was changed")
	}
}

func TestUserAPIAppliesDeclaredErrorMessages(t *testing.T) {
	api := newAPITest(t)

	missing := api.request(http.MethodGet, "/users/42", "", http.StatusNotFound)
	if missing.Body.String() != "user not found\n" {
		t.Fatalf("unexpected missing-user message: %q", missing.Body.String())
	}

	invalid := api.request(http.MethodPut, "/users/42", `{"name":" "}`, http.StatusBadRequest)
	if invalid.Body.String() != "user ID and name are required\n" {
		t.Fatalf("unexpected validation message: %q", invalid.Body.String())
	}
}
