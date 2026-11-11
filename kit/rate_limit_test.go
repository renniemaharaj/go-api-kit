package kit

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func testLimiter(t *testing.T, config RateLimitConfig) *RateLimiter {
	t.Helper()

	limiter, err := NewRateLimiter(config)
	if err != nil {
		t.Fatal(err)
	}

	return limiter
}

func TestRateLimitCannotBeBypassedByChangingRequests(t *testing.T) {
	limiter := testLimiter(t, RateLimitConfig{
		Requests: 1,
		Interval: time.Hour,
		Burst:    2,
	})
	visited := 0
	handler := limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		visited++
		w.WriteHeader(http.StatusNoContent)
	}))

	for index, target := range []string{
		"/users/42",
		"/users/42?random=1",
		"/users/57",
		"/files/new-file.txt?random=2",
		"/does-not-exist",
	} {
		request := httptest.NewRequest(http.MethodGet, target, nil)
		if index%2 == 1 {
			request.Method = http.MethodPost
		}

		request.RemoteAddr = fmt.Sprintf("192.0.2.1:%d", 1000+index)
		request.Header.Set("X-Forwarded-For", fmt.Sprintf("198.51.100.%d", index))
		request.Header.Set("X-Real-IP", fmt.Sprintf("203.0.113.%d", index))

		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)

		want := http.StatusNoContent
		if index >= 2 {
			want = http.StatusTooManyRequests
			if response.Header().Get("Retry-After") == "" {
				t.Fatal("rate-limited response has no Retry-After")
			}
		}

		if response.Code != want {
			t.Fatalf("request %d to %s: got %d, want %d", index, target, response.Code, want)
		}
	}

	if visited != 2 {
		t.Fatalf("downstream received %d requests, want 2", visited)
	}

	request := httptest.NewRequest(http.MethodGet, "/users/57", nil)
	request.RemoteAddr = "192.0.2.2:1000"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("different IP shared a bucket: %d", response.Code)
	}
}

func TestRateLimitRefillsAndBoundsClientMemory(t *testing.T) {
	limiter := testLimiter(t, RateLimitConfig{
		Requests:   1,
		Interval:   time.Second,
		Burst:      1,
		MaxClients: 1,
		IdleTTL:    time.Second,
	})
	now := time.Now()
	limiter.now = func() time.Time {
		return now
	}

	if allowed, _ := limiter.allow("first"); !allowed {
		t.Fatal("first request was rejected")
	}

	if allowed, _ := limiter.allow("first"); allowed {
		t.Fatal("burst limit was ignored")
	}

	if allowed, _ := limiter.allow("second"); allowed {
		t.Fatal("full client table admitted a new client")
	}

	now = now.Add(500 * time.Millisecond)
	if allowed, _ := limiter.allow("first"); allowed {
		t.Fatal("bucket refilled too early")
	}

	now = now.Add(500 * time.Millisecond)
	if allowed, _ := limiter.allow("first"); !allowed {
		t.Fatal("bucket did not refill")
	}

	now = now.Add(time.Second)
	if allowed, _ := limiter.allow("second"); !allowed {
		t.Fatal("idle bucket was not removed")
	}

	if len(limiter.clients) != 1 {
		t.Fatalf("client table has %d entries", len(limiter.clients))
	}
}

func TestRateLimitIsAtomicAcrossConcurrentRequests(t *testing.T) {
	limiter := testLimiter(t, RateLimitConfig{
		Requests: 1,
		Interval: time.Hour,
		Burst:    8,
	})

	var allowed atomic.Int64
	handler := limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		allowed.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))

	var requests sync.WaitGroup
	for index := range 100 {
		requests.Go(func() {
			request := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/users/%d", index), nil)
			request.RemoteAddr = "192.0.2.1:1234"
			handler.ServeHTTP(httptest.NewRecorder(), request)
		})
	}

	requests.Wait()
	if allowed.Load() != 8 {
		t.Fatalf("admitted %d concurrent requests, want 8", allowed.Load())
	}
}

func TestRateLimiterRejectsInvalidConfigurationAndAddresses(t *testing.T) {
	for _, config := range []RateLimitConfig{
		{},
		{
			Requests: 1,
			Interval: time.Second,
		},
		{
			Requests:   1,
			Interval:   time.Second,
			Burst:      1,
			MaxClients: -1,
		},
		{
			Requests: 1,
			Interval: time.Second,
			Burst:    2,
			IdleTTL:  time.Second,
		},
	} {
		if _, err := NewRateLimiter(config); err == nil {
			t.Fatalf("accepted invalid config: %#v", config)
		}
	}

	first, err := peerIP("[::ffff:192.0.2.1]:1234")
	if err != nil {
		t.Fatal(err)
	}

	second, err := peerIP("192.0.2.1:5678")
	if err != nil || first != second {
		t.Fatalf("mapped IPv4 address created another identity: %q, %q, %v", first, second, err)
	}

	if _, err := peerIP("invalid-address"); err == nil {
		t.Fatal("invalid address was accepted")
	}
}
