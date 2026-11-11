package kit

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func clearRateLimitEnvironment(t *testing.T) {
	t.Helper()

	for _, name := range []string{
		"RATE_LIMIT_REQUESTS",
		"RATE_LIMIT_INTERVAL",
		"RATE_LIMIT_BURST",
		"RATE_LIMIT_MAX_CLIENTS",
		"RATE_LIMIT_IDLE_TTL",
	} {
		t.Setenv(name, "")
		if err := os.Unsetenv(name); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRateLimitLoadsFileAndSharesAllowanceAcrossHandlers(t *testing.T) {
	clearRateLimitEnvironment(t)
	t.Chdir(t.TempDir())
	contents := "RATE_LIMIT_REQUESTS=1\nRATE_LIMIT_INTERVAL='1h'\nRATE_LIMIT_BURST=1\nRATE_LIMIT_MAX_CLIENTS=12\nRATE_LIMIT_IDLE_TTL=2h\n"
	if err := os.WriteFile(".env", []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	// The process overrides the file, and each attachment shares this kit.
	t.Setenv("RATE_LIMIT_BURST", "2")
	limit := RateLimit()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	first := limit(handler)
	second := limit(handler)

	for index, endpoint := range []http.Handler{first, second, first} {
		response := httptest.NewRecorder()
		endpoint.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))

		want := http.StatusNoContent
		if index == 2 {
			want = http.StatusTooManyRequests
		}

		if response.Code != want {
			t.Fatalf("request %d: got %d, want %d", index, response.Code, want)
		}
	}
}

func TestRateLimiterFromEnvDefaultsAndOverrides(t *testing.T) {
	clearRateLimitEnvironment(t)
	filename := filepath.Join(t.TempDir(), "missing.env")
	limiter, err := NewRateLimiterFromEnv(filename)
	if err != nil {
		t.Fatal(err)
	}

	if limiter.config.Requests != 60 || limiter.config.Interval != time.Second || limiter.config.Burst != 120 {
		t.Fatalf("unexpected defaults: %#v", limiter.config)
	}

	contents := "RATE_LIMIT_REQUESTS=7\nRATE_LIMIT_INTERVAL=2s\nRATE_LIMIT_BURST=9\nRATE_LIMIT_MAX_CLIENTS=12\nRATE_LIMIT_IDLE_TTL=10m\n"
	if err := os.WriteFile(filename, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	limiter, err = NewRateLimiterFromEnv(filename)
	if err != nil {
		t.Fatal(err)
	}

	want := RateLimitConfig{
		Requests:   7,
		Interval:   2 * time.Second,
		Burst:      9,
		MaxClients: 12,
		IdleTTL:    10 * time.Minute,
	}
	if limiter.config != want {
		t.Fatalf("got %#v, want %#v", limiter.config, want)
	}
}

func TestRateLimiterFromEnvRejectsInvalidSettings(t *testing.T) {
	clearRateLimitEnvironment(t)
	for _, test := range []struct {
		name  string
		value string
	}{
		{
			name:  "RATE_LIMIT_REQUESTS",
			value: "many",
		},
		{
			name:  "RATE_LIMIT_INTERVAL",
			value: "soon",
		},
		{
			name:  "RATE_LIMIT_BURST",
			value: "0",
		},
		{
			name:  "RATE_LIMIT_IDLE_TTL",
			value: "1ns",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv(test.name, test.value)
			filename := filepath.Join(t.TempDir(), "missing.env")
			if _, err := NewRateLimiterFromEnv(filename); err == nil {
				t.Fatal("invalid configuration was accepted")
			}
		})
	}
}

func TestRateLimitFailsAtConstructionOnInvalidConfiguration(t *testing.T) {
	clearRateLimitEnvironment(t)
	t.Chdir(t.TempDir())
	t.Setenv("RATE_LIMIT_REQUESTS", "invalid")

	defer func() {
		problem := recover()
		err, ok := problem.(error)
		if !ok || !strings.Contains(err.Error(), "RATE_LIMIT_REQUESTS") {
			t.Fatalf("got panic %v, want a configuration error", problem)
		}
	}()

	RateLimit()
}
