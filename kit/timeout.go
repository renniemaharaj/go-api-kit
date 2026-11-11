package kit

import (
	"net/http"
	"time"
)

// Timeout sets a request deadline and returns HTTP 503 when it expires.
// It buffers responses and does not support streaming or connection hijacking.
func Timeout(duration time.Duration) Middleware {
	if duration <= 0 {
		panic("kit: timeout must be positive")
	}

	return func(next http.Handler) http.Handler {
		return http.TimeoutHandler(next, duration, "request timed out\n")
	}
}
