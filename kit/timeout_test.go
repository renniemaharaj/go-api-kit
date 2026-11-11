package kit_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/renniemaharaj/go-api-kit/data"
	"github.com/renniemaharaj/go-api-kit/kit"
)

func TestRequestContextReachesAuthAndLoader(t *testing.T) {
	type requestKey struct{}
	ctx, cancel := context.WithTimeout(context.WithValue(context.Background(), requestKey{}, "request-value"), time.Second)
	defer cancel()
	parentDeadline, _ := ctx.Deadline()
	check := func(ctx context.Context) {
		t.Helper()

		deadline, ok := ctx.Deadline()
		if !ok || !deadline.Equal(parentDeadline) || ctx.Value(requestKey{}) != "request-value" {
			t.Errorf("request deadline or value lost: %v, %v", deadline, ok)
		}
	}

	users, err := data.NewCache(func(ctx context.Context, id string) (string, error) {
		check(ctx)

		principal, ok := kit.PrincipalFromContext(ctx)
		if !ok || principal.ID != "42" {
			t.Error("repository did not receive verified identity")
		}

		return "Ada", nil
	}, data.CacheConfig[string]{TTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}

	router := kit.New()
	router.Use(
		kit.Timeout(time.Minute),
		kit.Authenticate(func(ctx context.Context, request *http.Request) (kit.Principal, error) {
			check(ctx)
			return kit.Principal{ID: "42"}, nil
		}),
		kit.Authorize(func(ctx context.Context, principal kit.Principal, request *http.Request) error {
			check(ctx)
			return nil
		}),
	)
	router.Get("/users/{id}", func(h kit.Handle) {
		value, err := users.Get(h.Context(), h.Param("id"))
		if err != nil {
			h.Error.Internal("load failed")
			return
		}

		h.Text(http.StatusOK, value)
	})

	for range 2 {
		request := httptest.NewRequest(http.MethodGet, "/users/42", nil).WithContext(ctx)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusOK || response.Body.String() != "Ada" {
			t.Fatalf("got %d %q", response.Code, response.Body.String())
		}
	}
}

func TestTimeoutCancelsRepositoryAndRejectsLateWrites(t *testing.T) {
	observed := make(chan error, 1)
	release := make(chan struct{})
	done := make(chan struct{})
	unblock := sync.OnceFunc(func() {
		close(release)
	})
	t.Cleanup(func() {
		unblock()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("timed-out handler did not finish")
		}
	})

	users, err := data.NewCache(func(ctx context.Context, id string) (string, error) {
		<-ctx.Done()
		observed <- ctx.Err()
		<-release

		return "late", nil
	}, data.CacheConfig[string]{TTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}

	lateWrite := make(chan error, 1)
	handler := kit.Timeout(20 * time.Millisecond)(kit.HandlerFunc(func(h kit.Handle) {
		defer close(done)
		_, err := users.Get(h.Context(), "42")
		if !errors.Is(err, context.DeadlineExceeded) {
			lateWrite <- err

			return
		}

		_, err = h.Writer().Write([]byte("late response"))
		lateWrite <- err
	}))

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/users/42", nil))
	if response.Code != http.StatusServiceUnavailable || response.Body.String() != "request timed out\n" {
		t.Fatalf("got %d %q", response.Code, response.Body.String())
	}

	select {
	case err := <-observed:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("repository cancellation: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("repository did not receive cancellation")
	}

	// Finish the worker before cleanup to verify the timeout writer rejects it.
	unblock()
	select {
	case err := <-lateWrite:
		if !errors.Is(err, http.ErrHandlerTimeout) {
			t.Fatalf("late write returned %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("late handler did not finish")
	}
}
