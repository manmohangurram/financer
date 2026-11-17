// Package userservice implements profile reads/updates, password change,
// logout-all, and avatar storage.
package userservice

import (
	"os"
	"path/filepath"

	"golang.org/x/crypto/bcrypt"

	"github.com/mohan9182/financer/internal/auth"
	"github.com/mohan9182/financer/internal/errors"
	"github.com/mohan9182/financer/internal/model"
	"github.com/mohan9182/financer/internal/repo"
)

const maxAvatarBytes = 5 << 20

type Service struct {
	repo      repo.UserRepo
	jwt       *auth.JWT
	avatarDir string
}

func New(repo repo.UserRepo, jwt *auth.JWT, avatarDir string) *Service {
	return &Service{repo: repo, jwt: jwt, avatarDir: avatarDir}
}

func (s *Service) Profile(userID string) (*model.ProfileResponse, error) {
	user, err := s.repo.ByID(userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.NotFound("user not found")
	}
	return &model.ProfileResponse{
		UserID:    user.ID,
		Name:      orEmpty(user.Name),
		Email:     user.Email,
		AvatarURL: orEmpty(user.AvatarURL),
	}, nil
}

func (s *Service) UpdateProfile(userID, name, email, avatarURL string) (*model.ProfileResponse, error) {
	if name == "" && email == "" && avatarURL == "" {
		return nil, errors.BadRequest("nothing to update")
	}
	if err := s.repo.UpdateProfile(userID, name, email, avatarURL); err != nil {
		return nil, errors.Conflict("email already in use")
	}
	return s.Profile(userID)
}

func (s *Service) ChangePassword(userID, currentPassword, newPassword string) (*model.AuthResponse, error) {
	if newPassword == "" {
		return nil, errors.BadRequest("new password is required")
	}
	if len(newPassword) < 6 {
		return nil, errors.BadRequest("new password must be at least 6 characters")
	}
	user, err := s.repo.ByID(userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.NotFound("user not found")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(*user.PasswordHash), []byte(currentPassword)); err != nil {
		return nil, errors.BadRequest("current password is incorrect")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return nil, errors.Internal("failed to hash password")
	}
	ver, err := s.repo.ChangePassword(userID, string(hash))
	if err != nil {
		return nil, errors.Internal("failed to update password")
	}
	access, err := s.jwt.AccessToken(userID, user.Email, ver)
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
		Email:        user.Email,
		Name:         orEmpty(user.Name),
	}, nil
}

func (s *Service) LogoutAll(userID string) error {
	if err := s.repo.LogoutAll(userID); err != nil {
		return errors.Internal("failed to log out all sessions")
	}
	return nil
}

func (s *Service) SaveAvatar(userID string, data []byte) (*model.ProfileResponse, error) {
	if len(data) == 0 {
		return nil, errors.BadRequest("failed to read avatar")
	}
	if len(data) > maxAvatarBytes {
		return nil, errors.BadRequest("avatar must be 5MB or smaller")
	}
	ext, ok := sniffAvatar(data)
	if !ok {
		return nil, errors.BadRequest("avatar must be a png, jpeg, or webp image")
	}
	if err := os.MkdirAll(s.avatarDir, 0o755); err != nil {
		return nil, errors.Internal("failed to save avatar")
	}
	filename := userID + ext
	if err := os.WriteFile(filepath.Join(s.avatarDir, filename), data, 0o644); err != nil {
		return nil, errors.Internal("failed to save avatar")
	}
	avatarURL := "/avatars/" + filename
	if err := s.repo.UpdateProfile(userID, "", "", avatarURL); err != nil {
		return nil, errors.Internal("failed to update avatar")
	}
	return s.Profile(userID)
}

func orEmpty(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func sniffAvatar(data []byte) (string, bool) {
	switch {
	case len(data) >= 8 && string(data[:8]) == "\x89PNG\r\n\x1a\n":
		return ".png", true
	case len(data) >= 3 && data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff:
		return ".jpg", true
	case len(data) > 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP":
		return ".webp", true
	}
	return "", false
}
