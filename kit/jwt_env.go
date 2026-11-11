package kit

import (
	"fmt"

	"github.com/golang-jwt/jwt/v5"

	"github.com/renniemaharaj/go-api-kit/internal/environment"
)

// JWTConfigFromEnv loads and validates the same JWT settings as the default stack.
func JWTConfigFromEnv(filename string) (JWTConfig, error) {
	values, err := environment.Load(filename)
	if err != nil {
		return JWTConfig{}, err
	}

	config, err := jwtConfigFromEnvironment(values)
	if err != nil {
		return JWTConfig{}, err
	}

	if _, err := NewJWT(config); err != nil {
		return JWTConfig{}, err
	}

	return config, nil
}

func jwtConfigFromEnvironment(values *environment.Reader) (JWTConfig, error) {
	config := JWTConfig{
		Algorithm:     values.String("JWT_ALGORITHM", "HS256"),
		Key:           []byte(values.String("JWT_SECRET", "")),
		Issuer:        values.String("JWT_ISSUER", "go-api-kit-example"),
		Audience:      values.String("JWT_AUDIENCE", "users-api"),
		ConfigVersion: values.String("JWT_CONFIG_VERSION", "v1"),
		Leeway:        values.Duration("JWT_LEEWAY", 0),
		BindIP:        values.Bool("JWT_BIND_IP", false),
	}
	if err := values.Err(); err != nil {
		return JWTConfig{}, err
	}

	if config.Algorithm == "RS256" {
		key, err := jwt.ParseRSAPublicKeyFromPEM([]byte(values.String("JWT_PUBLIC_KEY", "")))
		if err != nil {
			return JWTConfig{}, fmt.Errorf("kit: JWT_PUBLIC_KEY must contain an RSA public key in PEM format")
		}

		config.Key = key
	}

	return config, nil
}
