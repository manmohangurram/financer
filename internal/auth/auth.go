// Package auth issues and validates HS256 JWT access/refresh tokens.
package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const issuer = "financer"

type Claims struct {
	UserID       string `json:"user_id"`
	Email        string `json:"email"`
	TokenVersion int64  `json:"ver"`
	Type         string `json:"typ"`
	jwt.RegisteredClaims
}

type JWT struct {
	secret []byte
}

func New(secret string) *JWT {
	return &JWT{secret: []byte(secret)}
}

func (j *JWT) AccessToken(userID, email string, ver int64) (string, error) {
	return j.token(userID, email, "access", ver, 24*time.Hour)
}

func (j *JWT) RefreshToken(userID string, ver int64) (string, error) {
	return j.token(userID, "", "refresh", ver, 30*24*time.Hour)
}

func (j *JWT) token(userID, email, typ string, ver int64, d time.Duration) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:       userID,
		Email:        email,
		TokenVersion: ver,
		Type:         typ,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(d)),
			IssuedAt:  jwt.NewNumericDate(now),
			Issuer:    issuer,
			Subject:   userID,
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(j.secret)
}

func (j *JWT) Validate(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return j.secret, nil
	}, jwt.WithIssuer(issuer))
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token claims")
	}
	return claims, nil
}
