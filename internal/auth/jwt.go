package auth

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims extends jwt.RegisteredClaims with a custom Username field
// for application-level user identification.
type Claims struct {
	jwt.RegisteredClaims
	Username string `json:"username"`
}

// GenerateToken creates a signed JWT for the given user with a 24-hour expiry.
// The token is signed with HS256 using the provided secret.
func GenerateToken(secret string, user *User) (string, error) {
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		Username: user.Username,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// ValidateToken parses and validates a JWT string, returning the embedded User
// on success. Returns an error if the token is expired, malformed, or
// signed with a different secret.
func ValidateToken(secret, tokenStr string) (*User, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(_ *jwt.Token) (any, error) {
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, jwt.ErrSignatureInvalid
	}
	return &User{Username: claims.Username}, nil
}
