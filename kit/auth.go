package kit

import (
	"context"
	"errors"
	"net/http"
	"slices"
)

var (
	ErrUnauthenticated = errors.New("authentication required")
	ErrForbidden       = errors.New("access forbidden")
)

// Principal contains an identity established by the application's verifier.
type Principal struct {
	ID    string
	Roles []string
}

type principalKey struct{}

// PrincipalFromContext returns the verified identity and a copy of its roles.
func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(principalKey{}).(Principal)
	principal.Roles = slices.Clone(principal.Roles)
	return principal, ok
}

// Authenticate verifies credentials on every request, including cache hits.
// Return ErrUnauthenticated for invalid credentials; other errors return HTTP 500.
func Authenticate(verify func(context.Context, *http.Request) (Principal, error)) Middleware {
	if verify == nil {
		panic("kit: authentication verifier is required")
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Context().Err() != nil {
				http.Error(w, "request canceled", http.StatusServiceUnavailable)
				return
			}

			principal, err := verify(r.Context(), r)
			if errors.Is(err, ErrUnauthenticated) || (err == nil && principal.ID == "") {
				w.Header().Set("WWW-Authenticate", `Bearer realm="api"`)
				http.Error(w, "authentication required", http.StatusUnauthorized)
				return
			}

			if err != nil {
				http.Error(w, "could not verify credentials", http.StatusInternalServerError)
				return
			}

			if r.Context().Err() != nil {
				http.Error(w, "request canceled", http.StatusServiceUnavailable)
				return
			}

			principal.Roles = slices.Clone(principal.Roles)
			ctx := context.WithValue(r.Context(), principalKey{}, principal)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// Authorize checks access using the verified identity and original request context.
// Return ErrForbidden to deny access; other errors return HTTP 500.
func Authorize(check func(context.Context, Principal, *http.Request) error) Middleware {
	if check == nil {
		panic("kit: authorization check is required")
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, ok := PrincipalFromContext(r.Context())
			if !ok {
				http.Error(w, "authentication required", http.StatusUnauthorized)
				return
			}

			if r.Context().Err() != nil {
				http.Error(w, "request canceled", http.StatusServiceUnavailable)
				return
			}

			err := check(r.Context(), principal, r)
			if errors.Is(err, ErrForbidden) {
				http.Error(w, "access forbidden", http.StatusForbidden)
				return
			}

			if err != nil {
				http.Error(w, "could not check permissions", http.StatusInternalServerError)
				return
			}

			if r.Context().Err() != nil {
				http.Error(w, "request canceled", http.StatusServiceUnavailable)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireAnyRole allows an authenticated principal with at least one listed role.
func RequireAnyRole(roles ...string) Middleware {
	if len(roles) == 0 {
		panic("kit: at least one role is required")
	}

	allowed := slices.Clone(roles)
	return Authorize(func(_ context.Context, principal Principal, _ *http.Request) error {
		for _, role := range allowed {
			if slices.Contains(principal.Roles, role) {
				return nil
			}
		}

		return ErrForbidden
	})
}

// RequireSubject binds a matched route parameter to the verified JWT subject.
// Install with Router.With so path parameters exist before this check runs.
func RequireSubject(parameter string) Middleware {
	if parameter == "" {
		panic("kit: subject path parameter is required")
	}

	return Authorize(func(_ context.Context, principal Principal, request *http.Request) error {
		id := request.PathValue(parameter)
		if id == "" || id != principal.ID {
			return ErrForbidden
		}

		return nil
	})
}
