package kit

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type corsPolicy struct {
	origins          map[string]bool
	methods          map[string]bool
	headers          map[string]bool
	allowedMethods   string
	allowedHeaders   string
	exposedHeaders   string
	allowCredentials bool
	maxAge           string
}

func (p corsPolicy) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		addVary(w.Header(), "Origin")

		origin := r.Header.Get("Origin")
		if origin == "" {
			next.ServeHTTP(w, r)
			return
		}

		if len(r.Header.Values("Origin")) != 1 || !validOrigin(origin) || (!p.origins[origin] && !p.origins["*"]) {
			http.Error(w, "origin not allowed", http.StatusForbidden)
			return
		}

		preflight := r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != ""
		method := r.Method
		if preflight {
			addVary(w.Header(), "Access-Control-Request-Method")
			addVary(w.Header(), "Access-Control-Request-Headers")

			method = r.Header.Get("Access-Control-Request-Method")
			if !allowedCORSHeaders(r.Header.Get("Access-Control-Request-Headers"), p.headers) {
				http.Error(w, "request headers not allowed", http.StatusForbidden)
				return
			}
		}

		if !p.methods[strings.ToLower(method)] {
			http.Error(w, "method not allowed by CORS", http.StatusForbidden)
			return
		}

		allowedOrigin := origin
		if p.origins["*"] {
			allowedOrigin = "*"
		}

		w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
		if p.allowCredentials {
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}

		if preflight {
			w.Header().Set("Access-Control-Allow-Methods", p.allowedMethods)
			w.Header().Set("Access-Control-Allow-Headers", p.allowedHeaders)
			if p.maxAge != "" {
				w.Header().Set("Access-Control-Max-Age", p.maxAge)
			}

			w.WriteHeader(http.StatusNoContent)
			return
		}

		if p.exposedHeaders != "" {
			w.Header().Set("Access-Control-Expose-Headers", p.exposedHeaders)
		}

		next.ServeHTTP(w, r)
	})
}

func validOrigin(origin string) bool {
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false
	}

	if parsed.Host == "" || parsed.User != nil {
		return false
	}

	return parsed.Path == "" && parsed.RawQuery == "" && parsed.Fragment == "" && !parsed.ForceQuery
}

func corsTokens(values []string) (map[string]bool, error) {
	tokens := make(map[string]bool)
	for _, value := range values {
		if value == "" {
			return nil, fmt.Errorf("kit: CORS tokens cannot be empty")
		}

		for _, char := range value {
			isLetter := (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z')
			isDigit := char >= '0' && char <= '9'
			isSymbol := strings.ContainsRune("!#$%&'*+-.^_`|~", char)
			if !isLetter && !isDigit && !isSymbol {
				return nil, fmt.Errorf("kit: invalid CORS token %q", value)
			}
		}

		tokens[strings.ToLower(value)] = true
	}

	return tokens, nil
}

func allowedCORSHeaders(requested string, allowed map[string]bool) bool {
	if requested == "" {
		return true
	}

	for _, header := range strings.Split(requested, ",") {
		if !allowed[strings.ToLower(strings.TrimSpace(header))] {
			return false
		}
	}

	return true
}

func addVary(header http.Header, value string) {
	for _, line := range header.Values("Vary") {
		for _, existing := range strings.Split(line, ",") {
			if strings.EqualFold(strings.TrimSpace(existing), value) {
				return
			}
		}
	}

	header.Add("Vary", value)
}
