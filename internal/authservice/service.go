// Package authservice implements signup / login / refresh.
package authservice

import (
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/mohan9182/financer/internal/auth"
	"github.com/mohan9182/financer/internal/errors"
	"github.com/mohan9182/financer/internal/model"
	"github.com/mohan9182/financer/internal/repo"
)

const (
	maxFailures = 5
	loginWindow = 15 * time.Minute
	maxTracked  = 10_000
)

type attempt struct {
	failures int
	first    time.Time
}

type Service struct {
	repo       repo.UserRepo
	jwt        *auth.JWT
	failuresMu sync.Mutex
	failures   map[string]*attempt
}

func New(repo repo.UserRepo, jwt *auth.JWT) *Service {
	return &Service{repo: repo, jwt: jwt, failures: make(map[string]*attempt)}
}

func (s *Service) Signup(email, password, name string) (*model.AuthResponse, error) {
	if email == "" || password == "" || name == "" {
		return nil, errors.BadRequest("email, password, and name are required")
	}
	if len(password) < 6 {
		return nil, errors.BadRequest("password must be at least 6 characters")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, errors.Internal("failed to hash password")
	}
	id, err := s.repo.Create(email, string(hash), name)
	if err != nil {
		return nil, err
	}
	return s.issueTokens(id, email, name)
}

func (s *Service) Login(email, password string) (*model.AuthResponse, error) {
	if email == "" || password == "" {
		return nil, errors.BadRequest("email and password are required")
	}
	key := strings.ToLower(email)
	if s.isThrottled(key) {
		return nil, errors.TooManyRequests("too many failed logins for this email; try again later")
	}
	user, err := s.repo.ByEmail(email)
	if err != nil {
		return nil, err
	}
	if user == nil {
		s.recordFailure(key)
		return nil, errors.Unauthorized("invalid credentials")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(*user.PasswordHash), []byte(password)); err != nil {
		s.recordFailure(key)
		return nil, errors.Unauthorized("invalid credentials")
	}
	s.failuresMu.Lock()
	delete(s.failures, key)
	s.failuresMu.Unlock()
	return s.issueTokens(user.ID, user.Email, *user.Name)
}

func (s *Service) RefreshToken(refreshToken string) (*model.AuthResponse, error) {
	if refreshToken == "" {
		return nil, errors.BadRequest("refresh_token is required")
	}
	claims, err := s.jwt.Validate(refreshToken)
	if err != nil {
		return nil, errors.Unauthorized("invalid refresh token")
	}
	if claims.Type != "refresh" {
		return nil, errors.Unauthorized("invalid refresh token")
	}
	user, err := s.repo.ByID(claims.UserID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.Unauthorized("user not found")
	}
	if claims.TokenVersion != user.TokenVersion {
		return nil, errors.Unauthorized("session has been revoked")
	}
	return s.issueTokens(user.ID, user.Email, *user.Name)
}

func (s *Service) isThrottled(key string) bool {
	s.failuresMu.Lock()
	defer s.failuresMu.Unlock()
	a, ok := s.failures[key]
	if !ok {
		return false
	}
	return a.failures >= maxFailures && time.Since(a.first) < loginWindow
}

func (s *Service) recordFailure(key string) {
	s.failuresMu.Lock()
	defer s.failuresMu.Unlock()
	if len(s.failures) >= maxTracked {
		s.failures = make(map[string]*attempt)
	}
	a, ok := s.failures[key]
	if !ok {
		a = &attempt{first: time.Now()}
		s.failures[key] = a
	}
	if time.Since(a.first) >= loginWindow {
		a = &attempt{first: time.Now()}
		s.failures[key] = a
	}
	a.failures++
}

func (s *Service) issueTokens(userID, email, name string) (*model.AuthResponse, error) {
	user, err := s.repo.ByID(userID)
	if err != nil || user == nil {
		return nil, errors.Internal("failed to read user")
	}
	ver := user.TokenVersion
	access, err := s.jwt.AccessToken(userID, email, ver)
	if err != nil {
		return nil, errors.Internal("failed to generate token")
	}
	refresh, err := s.jwt.RefreshToken(userID, ver)
	if err != nil {
		return nil, errors.Internal("failed to generate token")
	}
	return &model.AuthResponse{
		AccessToken:  access,
		RefreshToken: refresh,
		UserID:       userID,
		Email:        email,
		Name:         name,
	}, nil
}
