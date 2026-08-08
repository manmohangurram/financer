package services

import (
	"context"
	"database/sql"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/mohan9182/financer/api"
	"github.com/mohan9182/financer/auth"
	"golang.org/x/crypto/bcrypt"
)

type UserService struct {
	db        *sql.DB
	jwtSecret string
	avatarDir string
}

func NewUserService(db *sql.DB, jwtSecret, avatarDir string) *UserService {
	return &UserService{db: db, jwtSecret: jwtSecret, avatarDir: avatarDir}
}

func (s *UserService) GetProfile(ctx context.Context, userID string) (*api.ProfileResponse, error) {
	var name, email, avatarURL string
	err := s.db.QueryRow(
		`SELECT name, email, avatar_url FROM users WHERE id = ?`, userID,
	).Scan(&name, &email, &avatarURL)
	if err != nil {
		return nil, NotFound("user not found")
	}
	return &api.ProfileResponse{UserId: userID, Name: name, Email: email, AvatarUrl: avatarURL}, nil
}

func (s *UserService) UpdateProfile(ctx context.Context, userID string, msg *api.UpdateProfileRequest) (*api.ProfileResponse, error) {
	if msg.Name == "" && msg.Email == "" && msg.AvatarUrl == "" {
		return nil, BadRequest("nothing to update")
	}
	if _, err := s.db.Exec(
		`UPDATE users SET
			name = COALESCE(NULLIF(?, ''), name),
			email = COALESCE(NULLIF(?, ''), email),
			avatar_url = COALESCE(NULLIF(?, ''), avatar_url)
		 WHERE id = ?`,
		msg.Name, msg.Email, msg.AvatarUrl, userID,
	); err != nil {
		return nil, Conflict("email already in use")
	}
	return s.GetProfile(ctx, userID)
}

// ChangePassword verifies the current password, updates the hash, bumps
// token_version (revoking every other session), and re-issues tokens for this
// one (re-auth on change).
func (s *UserService) ChangePassword(ctx context.Context, userID string, msg *api.ChangePasswordRequest) (*api.AuthResponse, error) {
	if msg.NewPassword == "" {
		return nil, BadRequest("new password is required")
	}
	if len(msg.NewPassword) < 6 {
		return nil, BadRequest("new password must be at least 6 characters")
	}

	var passwordHash, email, name string
	err := s.db.QueryRow(
		`SELECT password_hash, email, name FROM users WHERE id = ?`, userID,
	).Scan(&passwordHash, &email, &name)
	if err != nil {
		return nil, NotFound("user not found")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(msg.CurrentPassword)); err != nil {
		return nil, BadRequest("current password is incorrect")
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(msg.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return nil, ServerError("internal error")
	}
	if _, err := s.db.Exec(
		`UPDATE users SET password_hash = ?, token_version = token_version + 1 WHERE id = ?`,
		string(hashed), userID,
	); err != nil {
		return nil, ServerError("failed to update password")
	}

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

// LogoutAll bumps token_version so every previously issued refresh token is
// rejected. Access tokens stay valid until expiry (24h).
// ponytail: stateless middleware, no per-request version check; 24h max
// lifetime is acceptable for a personal app. Add a sessions table if real
// revocation matters.
func (s *UserService) LogoutAll(ctx context.Context, userID string) (*api.OperationResponse, error) {
	if _, err := s.db.Exec(`UPDATE users SET token_version = token_version + 1 WHERE id = ?`, userID); err != nil {
		return nil, ServerError("failed to log out all sessions")
	}
	return &api.OperationResponse{Success: true, Message: "all sessions logged out"}, nil
}

// SaveAvatar stores an uploaded image (sniffed content type, 5MB cap) and
// updates avatar_url to the served path.
func (s *UserService) SaveAvatar(ctx context.Context, userID string, r io.Reader) (*api.ProfileResponse, error) {
	data, err := io.ReadAll(io.LimitReader(r, 5<<20+1))
	if err != nil {
		return nil, BadRequest("failed to read avatar")
	}
	if len(data) > 5<<20 {
		return nil, BadRequest("avatar must be 5MB or smaller")
	}
	sniffed := http.DetectContentType(data)
	ext := map[string]string{
		"image/png":  ".png",
		"image/jpeg": ".jpg",
		"image/webp": ".webp",
	}[sniffed]
	if ext == "" {
		return nil, BadRequest("avatar must be a png, jpeg, or webp image")
	}

	if err := os.MkdirAll(s.avatarDir, 0o755); err != nil {
		return nil, ServerError("failed to save avatar")
	}
	filename := userID + ext
	if err := os.WriteFile(filepath.Join(s.avatarDir, filename), data, 0o644); err != nil {
		return nil, ServerError("failed to save avatar")
	}
	avatarURL := "/avatars/" + filename

	if _, err := s.db.Exec(`UPDATE users SET avatar_url = ? WHERE id = ?`, avatarURL, userID); err != nil {
		return nil, ServerError("failed to update avatar")
	}
	return s.GetProfile(ctx, userID)
}
