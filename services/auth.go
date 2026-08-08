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

	accessToken, err := auth.GenerateAccessToken(userID, msg.Email, s.jwtSecret)
	if err != nil {
		return nil, ServerError("failed to generate token")
	}

	refreshToken, err := auth.GenerateRefreshToken(userID, s.jwtSecret)
	if err != nil {
		return nil, ServerError("failed to generate token")
	}

	return &api.AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		UserId:       userID,
		Email:        msg.Email,
		Name:         msg.Name,
	}, nil
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

	accessToken, err := auth.GenerateAccessToken(userID, msg.Email, s.jwtSecret)
	if err != nil {
		return nil, ServerError("failed to generate token")
	}

	refreshToken, err := auth.GenerateRefreshToken(userID, s.jwtSecret)
	if err != nil {
		return nil, ServerError("failed to generate token")
	}

	return &api.AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		UserId:       userID,
		Email:        msg.Email,
		Name:         name,
	}, nil
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
	err = s.db.QueryRow(
		`SELECT email, name FROM users WHERE id = ?`, claims.Subject,
	).Scan(&email, &name)
	if err != nil {
		return nil, Unauthorized("user not found")
	}

	accessToken, err := auth.GenerateAccessToken(claims.Subject, email, s.jwtSecret)
	if err != nil {
		return nil, ServerError("failed to generate token")
	}

	refreshToken, err := auth.GenerateRefreshToken(claims.Subject, s.jwtSecret)
	if err != nil {
		return nil, ServerError("failed to generate token")
	}

	return &api.AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		UserId:       claims.Subject,
		Email:        email,
		Name:         name,
	}, nil
}

func (s *AuthService) GetMe(ctx context.Context, userID string, _ *api.GetMeRequest) (*api.GetMeResponse, error) {
	var email, name string
	err := s.db.QueryRow(
		`SELECT email, name FROM users WHERE id = ?`, userID,
	).Scan(&email, &name)
	if err != nil {
		return nil, NotFound("user not found")
	}

	return &api.GetMeResponse{
		UserId: userID,
		Email:  email,
		Name:   name,
	}, nil
}