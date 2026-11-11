package kit

import (
	"context"
	"crypto/rsa"
	"fmt"
	"math/big"
	"net/http"
	"net/netip"
	"slices"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWTClaims binds a signed subject to roles and an application configuration.
// UserID, when present, must equal Subject. IP is required when BindIP is enabled.
type JWTClaims struct {
	jwt.RegisteredClaims
	UserID        string   `json:"user_id,omitempty"`
	Roles         []string `json:"roles,omitempty"`
	ConfigVersion string   `json:"config_version"`
	IP            string   `json:"ip,omitempty"`
}

// JWTConfig pins the signing method, verification key, and expected claims.
// Algorithm must be HS256 with a 32-byte secret or RS256 with a 2048-bit public key.
type JWTConfig struct {
	Algorithm     string
	Key           any
	Issuer        string
	Audience      string
	ConfigVersion string
	Leeway        time.Duration
	BindIP        bool
}

func NewJWT(config JWTConfig) (Middleware, error) {
	if strings.TrimSpace(config.Issuer) == "" {
		return nil, fmt.Errorf("kit: JWT issuer is required")
	}

	if strings.TrimSpace(config.Audience) == "" {
		return nil, fmt.Errorf("kit: JWT audience is required")
	}

	if strings.TrimSpace(config.ConfigVersion) == "" {
		return nil, fmt.Errorf("kit: JWT config version is required")
	}

	if config.Leeway < 0 {
		return nil, fmt.Errorf("kit: JWT leeway cannot be negative")
	}

	key, err := jwtVerificationKey(config)
	if err != nil {
		return nil, err
	}

	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{config.Algorithm}),
		jwt.WithIssuer(config.Issuer),
		jwt.WithAudience(config.Audience),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(config.Leeway),
		jwt.WithStrictDecoding(),
	)

	verifier := jwtVerifier{
		parser: parser,
		key:    key,
		config: config,
	}

	return Authenticate(verifier.verify), nil
}

type jwtVerifier struct {
	parser *jwt.Parser
	key    any
	config JWTConfig
}

func (v jwtVerifier) verify(ctx context.Context, request *http.Request) (Principal, error) {
	if err := ctx.Err(); err != nil {
		return Principal{}, err
	}

	headers := request.Header.Values("Authorization")
	if len(headers) != 1 || len(headers[0]) > 8192 {
		return Principal{}, ErrUnauthenticated
	}

	parts := strings.Fields(headers[0])
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return Principal{}, ErrUnauthenticated
	}

	claims := &JWTClaims{}
	token, err := v.parser.ParseWithClaims(parts[1], claims, func(_ *jwt.Token) (any, error) {
		return v.key, nil
	})
	if err != nil || !token.Valid || strings.TrimSpace(claims.Subject) == "" {
		return Principal{}, ErrUnauthenticated
	}

	if claims.ConfigVersion != v.config.ConfigVersion {
		return Principal{}, ErrUnauthenticated
	}

	if claims.UserID != "" && claims.UserID != claims.Subject {
		return Principal{}, ErrUnauthenticated
	}

	if v.config.BindIP {
		claimedIP, err := netip.ParseAddr(claims.IP)
		if err != nil {
			return Principal{}, ErrUnauthenticated
		}

		actualIP, err := peerIP(request.RemoteAddr)
		if err != nil || claimedIP.Unmap().WithZone("").String() != actualIP {
			return Principal{}, ErrUnauthenticated
		}
	}

	return Principal{
		ID:    claims.Subject,
		Roles: claims.Roles,
	}, nil
}

func jwtVerificationKey(config JWTConfig) (any, error) {
	switch config.Algorithm {
	case jwt.SigningMethodHS256.Alg():
		key, ok := config.Key.([]byte)
		if !ok || len(key) < 32 {
			return nil, fmt.Errorf("kit: HS256 requires a secret of at least 32 bytes")
		}

		return slices.Clone(key), nil
	case jwt.SigningMethodRS256.Alg():
		key, ok := config.Key.(*rsa.PublicKey)
		if !ok || key == nil || key.N == nil || key.N.BitLen() < 2048 || key.E < 3 || key.E%2 == 0 {
			return nil, fmt.Errorf("kit: RS256 requires an RSA public key of at least 2048 bits")
		}

		return &rsa.PublicKey{
			N: new(big.Int).Set(key.N),
			E: key.E,
		}, nil
	default:
		return nil, fmt.Errorf("kit: JWT algorithm must be HS256 or RS256")
	}
}
