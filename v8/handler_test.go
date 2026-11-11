package v8

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/renniemaharaj/go-api-kit/kit"
)

type serviceFunc func(context.Context) (Status, error)

func (check serviceFunc) Check(ctx context.Context) (Status, error) {
	return check(ctx)
}

func TestHandlerTranslatesServiceResults(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		for _, healthy := range []bool{true, false} {
			request := httptest.NewRequest(method, "/health", nil)
			calls := 0
			handlers := handler{service: serviceFunc(func(ctx context.Context) (Status, error) {
				calls++
				if ctx != request.Context() {
					t.Fatal("service did not receive the request context")
				}

				if !healthy {
					return Status{}, errors.New("private storage details")
				}

				return Status{Status: "ok", UptimeSeconds: 42}, nil
			})}

			response := httptest.NewRecorder()
			kit.NewHandler(handlers.Check).ServeHTTP(response, request)

			wantCode := http.StatusOK
			wantStatus := Status{Status: "ok", UptimeSeconds: 42}
			if !healthy {
				wantCode = http.StatusServiceUnavailable
				wantStatus = Status{Status: "unavailable"}
			}

			if response.Code != wantCode || calls != 1 {
				t.Fatalf("%s healthy=%t: status=%d, service calls=%d", method, healthy, response.Code, calls)
			}
			if response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("Content-Type") != "application/json; charset=utf-8" {
				t.Fatalf("unexpected health headers: %v", response.Header())
			}

			if method == http.MethodHead {
				if response.Body.Len() != 0 {
					t.Fatal("HEAD returned a body")
				}
				continue
			}

			var status Status
			if err := json.Unmarshal(response.Body.Bytes(), &status); err != nil {
				t.Fatal(err)
			}
			if status != wantStatus {
				t.Fatalf("unexpected response: %s", response.Body.String())
			}
		}
	}
}
