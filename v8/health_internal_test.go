package v8

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthHonorsCanceledRequests(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		response := httptest.NewRecorder()
		request := httptest.NewRequest(method, "/health", nil).WithContext(ctx)
		healthHandler().ServeHTTP(response, request)
		if response.Code != http.StatusServiceUnavailable || response.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("unexpected canceled health response: %d %v", response.Code, response.Header())
		}

		if method == http.MethodHead {
			if response.Body.Len() != 0 {
				t.Fatal("HEAD returned a body")
			}
			continue
		}

		var result struct {
			Status string `json:"status"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || result.Status != "unavailable" {
			t.Fatalf("unexpected canceled health body: %q, %v", response.Body.String(), err)
		}
	}
}
