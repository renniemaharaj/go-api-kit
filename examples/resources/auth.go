package main

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/renniemaharaj/go-api-kit/kit"
)

// issueToken is a local development helper, separate from the HTTP API.
func issueToken(id string) (string, error) {
	config, err := kit.JWTConfigFromEnv(".env")
	if err != nil {
		return "", err
	}

	if config.Algorithm != "HS256" {
		return "", fmt.Errorf("development token issuance requires JWT_ALGORITHM=HS256")
	}

	now := time.Now()
	claims := kit.JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    config.Issuer,
			Audience:  jwt.ClaimStrings{config.Audience},
			Subject:   id,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(15 * time.Minute)),
		},
		UserID:        id,
		Roles:         []string{"editor"},
		ConfigVersion: config.ConfigVersion,
	}

	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(config.Key)
}
