package services

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/mohan9182/financer/auth"
	"github.com/mohan9182/financer/api"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	db        *sql.DB
	jwtSecret string
}

func NewAuthService(db *sql.DB, jwtSecret string) *AuthService {
	return &AuthService{db: db, jwtSecret: jwtSecret}
}

func (s *AuthService) Signup(ctx context.Context, msg *api.SignupRequest) (*api.AuthResponse, error) {
	if msg.Email == "" || msg.Password == "" || msg.Name == "" {
		return nil, BadRequest("email, password, and name are required")
	}
	if len(msg.Password) < 6 {
		return nil, BadRequest("password must be at least 6 characters")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(msg.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, ServerError("internal error")
	}

	userID := uuid.New().String()
	_, err = s.db.Exec(
		`INSERT INTO users (id, email, password_hash, name, created_at) VALUES (?, ?, ?, ?, ?)`,
		userID, msg.Email, string(hashedPassword), msg.Name, time.Now().UTC(),
	)
	if err != nil {
		return nil, Conflict("email already exists")
	}

	return s.issueTokens(userID, msg.Email, msg.Name)
}

func (s *AuthService) Login(ctx context.Context, msg *api.LoginRequest) (*api.AuthResponse, error) {
	if msg.Email == "" || msg.Password == "" {
		return nil, BadRequest("email and password are required")
	}

	var userID, passwordHash, name string
	err := s.db.QueryRow(
		`SELECT id, password_hash, name FROM users WHERE email = ?`, msg.Email,
	).Scan(&userID, &passwordHash, &name)
	if err != nil {
		return nil, Unauthorized("invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(msg.Password)); err != nil {
		return nil, Unauthorized("invalid credentials")
	}

	return s.issueTokens(userID, msg.Email, name)
}

func (s *AuthService) RefreshToken(ctx context.Context, msg *api.RefreshTokenRequest) (*api.AuthResponse, error) {
	if msg.RefreshToken == "" {
		return nil, BadRequest("refresh_token is required")
	}

	claims, err := auth.ValidateToken(msg.RefreshToken, s.jwtSecret)
	if err != nil {
		return nil, Unauthorized("invalid refresh token")
	}

	var email, name string
	var tokenVersion int64
	err = s.db.QueryRow(
		`SELECT email, name, token_version FROM users WHERE id = ?`, claims.Subject,
	).Scan(&email, &name, &tokenVersion)
	if err != nil {
		return nil, Unauthorized("user not found")
	}

	if claims.TokenVersion != tokenVersion {
		return nil, Unauthorized("session has been revoked")
	}

	return s.issueTokens(claims.Subject, email, name)
}

// issueTokens builds the access/refresh pair for a user and returns the auth
// response carrying them.
func (s *AuthService) issueTokens(userID, email, name string) (*api.AuthResponse, error) {
	var tokenVersion int64
	if err := s.db.QueryRow(`SELECT token_version FROM users WHERE id = ?`, userID).Scan(&tokenVersion); err != nil {
		return nil, ServerError("failed to read user")
	}
	accessToken, err := auth.GenerateAccessToken(userID, email, s.jwtSecret, tokenVersion)
	if err != nil {
		return nil, ServerError("failed to generate token")
	}
	refreshToken, err := auth.GenerateRefreshToken(userID, s.jwtSecret, tokenVersion)
	if err != nil {
		return nil, ServerError("failed to generate token")
	}
	return &api.AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		UserId:       userID,
		Email:        email,
		Name:         name,
	}, nil
}