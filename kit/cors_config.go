package kit

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// CORSConfig permits specific browser origins, methods, and request headers.
// Wildcard origins cannot be combined with credentials.
type CORSConfig struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	ExposedHeaders   []string
	AllowCredentials bool
	MaxAge           time.Duration
}

func NewCORS(config CORSConfig) (Middleware, error) {
	if len(config.AllowedOrigins) == 0 || config.MaxAge < 0 {
		return nil, fmt.Errorf("kit: CORS origins are required and MaxAge cannot be negative")
	}

	origins := make(map[string]bool)
	for _, origin := range config.AllowedOrigins {
		if origin == "*" {
			if config.AllowCredentials {
				return nil, fmt.Errorf("kit: credentialed CORS requires explicit origins")
			}
		} else if !validOrigin(origin) {
			return nil, fmt.Errorf("kit: invalid CORS origin %q", origin)
		}

		origins[origin] = true
	}

	if len(config.AllowedMethods) == 0 {
		config.AllowedMethods = []string{
			"GET",
			"HEAD",
			"POST",
			"PUT",
			"PATCH",
			"DELETE",
			"OPTIONS",
		}
	}

	if len(config.AllowedHeaders) == 0 {
		config.AllowedHeaders = []string{
			"Authorization",
			"Content-Type",
		}
	}

	methods, err := corsTokens(config.AllowedMethods)
	if err != nil {
		return nil, err
	}

	headers, err := corsTokens(config.AllowedHeaders)
	if err != nil {
		return nil, err
	}

	if _, err := corsTokens(config.ExposedHeaders); err != nil {
		return nil, err
	}

	policy := corsPolicy{
		origins:          origins,
		methods:          methods,
		headers:          headers,
		allowedMethods:   strings.Join(config.AllowedMethods, ", "),
		allowedHeaders:   strings.Join(config.AllowedHeaders, ", "),
		exposedHeaders:   strings.Join(config.ExposedHeaders, ", "),
		allowCredentials: config.AllowCredentials,
	}
	if config.MaxAge > 0 {
		policy.maxAge = strconv.FormatInt(int64(config.MaxAge/time.Second), 10)
	}

	return policy.middleware, nil
}
