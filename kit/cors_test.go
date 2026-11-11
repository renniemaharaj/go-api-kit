package kit_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/renniemaharaj/go-api-kit/kit"
)

func TestCORSPreflightRunsBeforeAuthentication(t *testing.T) {
	cors, err := kit.NewCORS(kit.CORSConfig{
		AllowedOrigins: []string{"https://app.example"},
		AllowedMethods: []string{
			"GET",
			"PUT",
		},
		AllowCredentials: true,
		MaxAge:           time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}

	authCalls := 0
	handler := cors(kit.Authenticate(func(context.Context, *http.Request) (kit.Principal, error) {
		authCalls++

		return kit.Principal{}, kit.ErrUnauthenticated
	})(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("unauthenticated request reached handler")
	})))

	request := httptest.NewRequest(http.MethodOptions, "/users/42", nil)
	request.Header.Set("Origin", "https://app.example")
	request.Header.Set("Access-Control-Request-Method", "PUT")
	request.Header.Set("Access-Control-Request-Headers", "authorization, CONTENT-TYPE")

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent || authCalls != 0 {
		t.Fatalf("preflight required credentials: %d, auth calls=%d", response.Code, authCalls)
	}

	if response.Header().Get("Access-Control-Allow-Origin") != "https://app.example" || response.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Fatalf("invalid CORS headers: %v", response.Header())
	}

	if !strings.Contains(strings.Join(response.Header().Values("Vary"), ","), "Origin") {
		t.Fatal("CORS response did not vary by origin")
	}

	request = httptest.NewRequest(http.MethodGet, "/users/42", nil)
	request.Header.Set("Origin", "https://app.example")

	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized || authCalls != 1 || response.Header().Get("Access-Control-Allow-Origin") != "https://app.example" {
		t.Fatalf("actual request bypassed auth or lost CORS: %d %v", response.Code, response.Header())
	}
}

func TestCORSRejectsUnapprovedOriginsMethodsAndHeaders(t *testing.T) {
	cors, err := kit.NewCORS(kit.CORSConfig{
		AllowedOrigins: []string{"https://app.example"},
		AllowedMethods: []string{"GET"},
	})
	if err != nil {
		t.Fatal(err)
	}

	handler := cors(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	for _, test := range []struct {
		origin  string
		method  string
		headers string
	}{
		{
			origin: "https://evil.example",
			method: "GET",
		},
		{
			origin: "https://app.example.evil.example",
			method: "GET",
		},
		{
			origin: "https://app.example",
			method: "DELETE",
		},
		{
			origin:  "https://app.example",
			method:  "GET",
			headers: "X-Unapproved",
		},
		{
			origin: "null",
			method: "GET",
		},
	} {
		request := httptest.NewRequest(http.MethodOptions, "/", nil)
		request.Header.Set("Origin", test.origin)
		request.Header.Set("Access-Control-Request-Method", test.method)
		request.Header.Set("Access-Control-Request-Headers", test.headers)

		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusForbidden || response.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Fatalf("unapproved CORS request accepted: %d %v", response.Code, response.Header())
		}
	}

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusNoContent {
		t.Fatalf("non-CORS request rejected: %d", response.Code)
	}
}

func TestCORSRejectsUnsafeConfiguration(t *testing.T) {
	for _, config := range []kit.CORSConfig{
		{},
		{
			AllowedOrigins:   []string{"*"},
			AllowCredentials: true,
		},
		{AllowedOrigins: []string{"https://app.example/path"}},
		{
			AllowedOrigins: []string{"https://app.example"},
			AllowedHeaders: []string{"bad\r\nheader"},
		},
		{
			AllowedOrigins: []string{"https://app.example"},
			MaxAge:         -time.Second,
		},
	} {
		if _, err := kit.NewCORS(config); err == nil {
			t.Fatalf("unsafe CORS config accepted: %#v", config)
		}
	}
}
