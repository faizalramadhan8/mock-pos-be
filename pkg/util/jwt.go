package util

import (
	"github.com/golang-jwt/jwt/v5"
	"github.com/faizalramadhan/pos-be/internal/application/dto"
)

// UnmarshalClaims parses a JWT token string and populates the provided claims struct with the extracted claims.
// It takes two parameters: `secret` (string) representing the JWT secret key used for token verification,
// and `tokenString` (string) representing the JWT token string to be parsed.
// The function returns a pointer to an `entities.UserClaims` struct containing the extracted claims or an error if the token parsing fails.
func UnmarshalClaims(secret, tokenString string) (*dto.JWTClaims, error) {
	// Create a new empty claims struct to be populated
	var claims dto.JWTClaims

	// Parse the JWT token with the provided claims struct and secret key
	token, err := jwt.ParseWithClaims(tokenString, &claims, func(t *jwt.Token) (interface{}, error) {
		// Return the secret key as the verification key
		return []byte(secret), nil
	})

	if err != nil {
		return nil, err
	}

	// Check if the token is valid
	if !token.Valid {
		return nil, jwt.ErrTokenNotValidYet
	}

	// Return the populated claims struct
	return &claims, nil
}

// MarshalClaims generates a JWT token string based on the provided secret key and claims.
// It returns the JWT token and any error encountered during token generation.
func MarshalClaims(secret string, claims *dto.JWTClaims) (*dto.JWTToken, error) {
	// Create a new JWT token with the specified signing method and claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Sign the token with the secret key
	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		return nil, err
	}

	// Prepare the output struct with the token string and expiration time
	output := &dto.JWTToken{
		TokenString: tokenString,
		ExpiredAt:   claims.ExpiresAt.Time,
	}

	return output, nil
}
