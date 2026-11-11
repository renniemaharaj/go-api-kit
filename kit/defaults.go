package kit

import (
	"fmt"
	"time"

	"github.com/renniemaharaj/go-api-kit/internal/environment"
)

// DefaultMiddleware builds the environment-driven stack once at startup.
// Order is rate limit, deadline, optional CORS, JWT authentication, and roles.
func DefaultMiddleware() ([]Middleware, error) {
	values, err := environment.Load(".env")
	if err != nil {
		return nil, err
	}

	limiter, err := newRateLimiterFromEnvironment(values)
	if err != nil {
		return nil, err
	}

	timeout := values.Duration("REQUEST_TIMEOUT", 15*time.Second)
	if err := values.Err(); err != nil {
		return nil, err
	}

	if timeout <= 0 {
		return nil, fmt.Errorf("kit: REQUEST_TIMEOUT must be positive")
	}

	stack := []Middleware{limiter.Middleware, Timeout(timeout)}
	if len(values.List("CORS_ALLOWED_ORIGINS")) > 0 {
		cors, err := corsFromEnvironment(values)
		if err != nil {
			return nil, err
		}

		stack = append(stack, cors)
	}

	hasKey := values.String("JWT_SECRET", "") != "" || values.String("JWT_PUBLIC_KEY", "") != ""
	enabled := values.Bool("JWT_ENABLED", hasKey)
	roles := values.List("JWT_REQUIRED_ROLES")
	if err := values.Err(); err != nil {
		return nil, err
	}

	if !enabled && len(roles) > 0 {
		return nil, fmt.Errorf("kit: JWT_REQUIRED_ROLES requires JWT authentication")
	}

	if enabled {
		config, err := jwtConfigFromEnvironment(values)
		if err != nil {
			return nil, err
		}

		authenticate, err := NewJWT(config)
		if err != nil {
			return nil, err
		}

		stack = append(stack, authenticate)
		if len(roles) > 0 {
			stack = append(stack, RequireAnyRole(roles...))
		}
	}

	return stack, nil
}

func corsFromEnvironment(values *environment.Reader) (Middleware, error) {
	config := CORSConfig{
		AllowedOrigins:   values.List("CORS_ALLOWED_ORIGINS"),
		AllowedMethods:   values.List("CORS_ALLOWED_METHODS"),
		AllowedHeaders:   values.List("CORS_ALLOWED_HEADERS"),
		ExposedHeaders:   values.List("CORS_EXPOSED_HEADERS", "Retry-After"),
		AllowCredentials: values.Bool("CORS_ALLOW_CREDENTIALS", false),
		MaxAge:           values.Duration("CORS_MAX_AGE", 10*time.Minute),
	}
	if err := values.Err(); err != nil {
		return nil, err
	}

	return NewCORS(config)
}
