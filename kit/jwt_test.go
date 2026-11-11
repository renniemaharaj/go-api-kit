package kit_test

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/renniemaharaj/go-api-kit/kit"
)

func jwtTestConfig() kit.JWTConfig {
	return kit.JWTConfig{
		Algorithm:     "HS256",
		Key:           []byte("test-only-secret-with-at-least-32-bytes"),
		Issuer:        "test-issuer",
		Audience:      "test-api",
		ConfigVersion: "v1",
	}
}

func jwtTestClaims() kit.JWTClaims {
	return kit.JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "test-issuer",
			Audience:  jwt.ClaimStrings{"test-api"},
			Subject:   "42",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-time.Minute)),
		},
		UserID:        "42",
		Roles:         []string{"editor"},
		ConfigVersion: "v1",
	}
}

func signJWT(t *testing.T, method jwt.SigningMethod, claims kit.JWTClaims, key any) string {
	t.Helper()

	token, err := jwt.NewWithClaims(method, claims).SignedString(key)
	if err != nil {
		t.Fatal(err)
	}

	return token
}

func jwtResponse(handler http.Handler, token, target, address string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodGet, target, nil)
	request.RemoteAddr = address
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("X-Forwarded-For", "192.0.2.1")

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func TestJWTRejectsInvalidSignaturesAndClaims(t *testing.T) {
	config := jwtTestConfig()
	authenticate, err := kit.NewJWT(config)
	if err != nil {
		t.Fatal(err)
	}

	handler := authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	for _, test := range []struct {
		name   string
		change func(*kit.JWTClaims)
	}{
		{
			name: "expired",
			change: func(c *kit.JWTClaims) {
				c.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Hour))
			},
		},
		{
			name: "missing expiration",
			change: func(c *kit.JWTClaims) {
				c.ExpiresAt = nil
			},
		},
		{
			name: "missing subject",
			change: func(c *kit.JWTClaims) {
				c.Subject = ""
			},
		},
		{
			name: "wrong issuer",
			change: func(c *kit.JWTClaims) {
				c.Issuer = "another-issuer"
			},
		},
		{
			name: "wrong audience",
			change: func(c *kit.JWTClaims) {
				c.Audience = jwt.ClaimStrings{"another-api"}
			},
		},
		{
			name: "wrong config",
			change: func(c *kit.JWTClaims) {
				c.ConfigVersion = "v2"
			},
		},
		{
			name: "conflicting user",
			change: func(c *kit.JWTClaims) {
				c.UserID = "57"
			},
		},
		{
			name: "not yet valid",
			change: func(c *kit.JWTClaims) {
				c.NotBefore = jwt.NewNumericDate(time.Now().Add(time.Hour))
			},
		},
		{
			name: "future issuance",
			change: func(c *kit.JWTClaims) {
				c.IssuedAt = jwt.NewNumericDate(time.Now().Add(time.Hour))
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			claims := jwtTestClaims()
			test.change(&claims)

			token := signJWT(t, jwt.SigningMethodHS256, claims, config.Key)
			response := jwtResponse(handler, token, "/", "192.0.2.1:1234")
			if response.Code != http.StatusUnauthorized {
				t.Fatalf("invalid JWT accepted: %d %q", response.Code, response.Body.String())
			}
		})
	}

	valid := signJWT(t, jwt.SigningMethodHS256, jwtTestClaims(), config.Key)
	parts := strings.Split(valid, ".")
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatal(err)
	}

	parts[1] = base64.RawURLEncoding.EncodeToString([]byte(strings.ReplaceAll(string(payload), "42", "57")))

	for _, token := range []string{
		"malformed.token",
		strings.Join(parts, "."),
		signJWT(t, jwt.SigningMethodHS256, jwtTestClaims(), []byte("different-key-with-at-least-32-bytes")),
		signJWT(t, jwt.SigningMethodHS384, jwtTestClaims(), config.Key),
		signJWT(t, jwt.SigningMethodNone, jwtTestClaims(), jwt.UnsafeAllowNoneSignatureType),
	} {
		if response := jwtResponse(handler, token, "/", "192.0.2.1:1234"); response.Code != http.StatusUnauthorized {
			t.Fatalf("tampered token accepted: %d", response.Code)
		}
	}

	if response := jwtResponse(handler, valid, "/", "192.0.2.1:1234"); response.Code != http.StatusNoContent {
		t.Fatalf("valid token rejected: %d", response.Code)
	}
}

func TestJWTSubjectMustMatchRouteID(t *testing.T) {
	config := jwtTestConfig()
	authenticate, err := kit.NewJWT(config)
	if err != nil {
		t.Fatal(err)
	}

	router := kit.New()
	router.Use(authenticate)
	router.With(kit.RequireSubject("id")).Get("/users/{id}", func(h kit.Handle) {
		h.NoContent()
	})

	token := signJWT(t, jwt.SigningMethodHS256, jwtTestClaims(), config.Key)
	for _, test := range []struct {
		target string
		status int
	}{
		{
			target: "/users/42",
			status: http.StatusNoContent,
		},
		{
			target: "/users/57",
			status: http.StatusForbidden,
		},
		{
			target: "/users/57?id=42",
			status: http.StatusForbidden,
		},
	} {
		response := jwtResponse(router, token, test.target, "192.0.2.1:1234")
		if response.Code != test.status {
			t.Fatalf("%s: got %d, want %d", test.target, response.Code, test.status)
		}
	}
}

func TestJWTIPBindingIgnoresSpoofedForwardedHeaders(t *testing.T) {
	config := jwtTestConfig()
	config.BindIP = true
	authenticate, err := kit.NewJWT(config)
	if err != nil {
		t.Fatal(err)
	}

	handler := authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	claims := jwtTestClaims()
	claims.IP = "192.0.2.1"
	token := signJWT(t, jwt.SigningMethodHS256, claims, config.Key)
	if response := jwtResponse(handler, token, "/", "192.0.2.1:1234"); response.Code != http.StatusNoContent {
		t.Fatalf("matching IP rejected: %d", response.Code)
	}

	if response := jwtResponse(handler, token, "/", "192.0.2.2:1234"); response.Code != http.StatusUnauthorized {
		t.Fatalf("spoofed forwarded header bypassed IP binding: %d", response.Code)
	}

	claims.IP = ""
	token = signJWT(t, jwt.SigningMethodHS256, claims, config.Key)
	if response := jwtResponse(handler, token, "/", "192.0.2.1:1234"); response.Code != http.StatusUnauthorized {
		t.Fatalf("missing IP claim accepted: %d", response.Code)
	}
}

func TestJWTRSAVerificationAndAlgorithmPinning(t *testing.T) {
	private, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	config := jwtTestConfig()
	config.Algorithm = "RS256"
	config.Key = &private.PublicKey
	authenticate, err := kit.NewJWT(config)
	if err != nil {
		t.Fatal(err)
	}

	handler := authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	token := signJWT(t, jwt.SigningMethodRS256, jwtTestClaims(), private)
	if response := jwtResponse(handler, token, "/", "192.0.2.1:1234"); response.Code != http.StatusNoContent {
		t.Fatalf("valid RSA token rejected: %d", response.Code)
	}

	forged := signJWT(t, jwt.SigningMethodHS256, jwtTestClaims(), private.PublicKey.N.Bytes())
	if response := jwtResponse(handler, forged, "/", "192.0.2.1:1234"); response.Code != http.StatusUnauthorized {
		t.Fatalf("algorithm confusion accepted: %d", response.Code)
	}
}

func TestJWTRejectsUnsafeConfiguration(t *testing.T) {
	for _, change := range []func(*kit.JWTConfig){
		func(c *kit.JWTConfig) {
			c.Algorithm = "none"
		},
		func(c *kit.JWTConfig) {
			c.Key = []byte("short")
		},
		func(c *kit.JWTConfig) {
			c.Issuer = ""
		},
		func(c *kit.JWTConfig) {
			c.Audience = ""
		},
		func(c *kit.JWTConfig) {
			c.ConfigVersion = ""
		},
		func(c *kit.JWTConfig) {
			c.Leeway = -time.Second
		},
		func(c *kit.JWTConfig) {
			c.Algorithm = "RS256"
		},
	} {
		config := jwtTestConfig()
		change(&config)
		if _, err := kit.NewJWT(config); err == nil {
			t.Fatalf("unsafe config accepted: %#v", config)
		}
	}
}
