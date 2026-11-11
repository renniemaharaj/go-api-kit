package kit

import (
	"time"

	"github.com/renniemaharaj/go-api-kit/internal/environment"
)

// RateLimit loads .env once at construction and returns shared IP middleware.
// Process variables take precedence. Invalid settings panic before serving.
func RateLimit() Middleware {
	limiter, err := NewRateLimiterFromEnv(".env")
	if err != nil {
		panic(err)
	}

	return limiter.Middleware
}

// NewRateLimiterFromEnv is the error-returning alternative to RateLimit.
func NewRateLimiterFromEnv(filename string) (*RateLimiter, error) {
	values, err := environment.Load(filename)
	if err != nil {
		return nil, err
	}

	return newRateLimiterFromEnvironment(values)
}

func newRateLimiterFromEnvironment(values *environment.Reader) (*RateLimiter, error) {
	config := RateLimitConfig{
		Requests:   values.Int("RATE_LIMIT_REQUESTS", 60),
		Interval:   values.Duration("RATE_LIMIT_INTERVAL", time.Second),
		Burst:      values.Int("RATE_LIMIT_BURST", 120),
		MaxClients: values.Int("RATE_LIMIT_MAX_CLIENTS", 4096),
		IdleTTL:    values.Duration("RATE_LIMIT_IDLE_TTL", 0),
	}
	if err := values.Err(); err != nil {
		return nil, err
	}

	return NewRateLimiter(config)
}
