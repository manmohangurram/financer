// Package keyservice manages per-user API keys: create (shown once), list
// (masked), revoke, and authenticate for the MCP server.
package keyservice

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"

	"github.com/mohan9182/financer/internal/errors"
	"github.com/mohan9182/financer/internal/model"
	"github.com/mohan9182/financer/internal/repo"
	tim "github.com/mohan9182/financer/internal/time"
)

const (
	maxKeysPerUser = 50
	keyPrefix      = "fin_live_"
)

type Service struct {
	repo repo.UserKeyRepo
}

func New(r repo.UserKeyRepo) *Service {
	return &Service{repo: r}
}

// Create issues a new key. expiresInDays of 0 leaves it non-expiring; scope is
// "read" (default) or "read_write".
func (s *Service) Create(userID, name string, expiresInDays int64, scope string) (*model.CreatedUserKey, error) {
	count, err := s.repo.Count(userID)
	if err != nil {
		return nil, err
	}
	if count >= maxKeysPerUser {
		return nil, errors.BadRequest("max 50 keys per user")
	}
	if strings.EqualFold(scope, "read_write") {
		scope = "read_write"
	} else {
		scope = "read"
	}
	key, hash, prefix := generateKey()
	var expiresAt *string
	if expiresInDays > 0 {
		v := tim.FromUTC(tim.NowUTC().AddDate(0, 0, int(expiresInDays)))
		expiresAt = &v
	}
	now := tim.NowGoTS()
	id, err := s.repo.Create(userID, name, hash, prefix, expiresAt, scope)
	if err != nil {
		return nil, err
	}
	return &model.CreatedUserKey{
		ID: id, Name: name, Key: key, KeyPrefix: prefix,
		CreatedAt: now, ExpiresAt: expiresAt, Scope: scope,
	}, nil
}

func (s *Service) List(userID string) ([]*model.UserKey, error) {
	rows, err := s.repo.List(userID)
	if err != nil {
		return nil, err
	}
	out := make([]*model.UserKey, 0, len(rows))
	for _, row := range rows {
		out = append(out, &model.UserKey{
			ID: row.ID, Name: row.Name, KeyPrefix: row.KeyPrefix,
			CreatedAt: row.CreatedAt, ExpiresAt: row.ExpiresAt,
			LastUsedAt: row.LastUsedAt, Scope: row.Scope,
		})
	}
	return out, nil
}

func (s *Service) Delete(userID, id string) error {
	ok, err := s.repo.Delete(userID, id)
	if err != nil {
		return err
	}
	if !ok {
		return errors.NotFound("key " + id + " not found")
	}
	return nil
}

// Authenticate validates a raw key for the MCP server and returns
// (userID, scope). Unknown and expired keys are rejected.
func (s *Service) Authenticate(key string) (string, string, error) {
	hash := hashKey(key)
	userID, id, scope, ok, err := s.repo.ByKeyHash(hash)
	if err != nil {
		return "", "", err
	}
	if !ok {
		return "", "", errors.Unauthorized("invalid API key")
	}
	row, err := s.repo.GetByID(userID, id)
	if err != nil {
		return "", "", err
	}
	if row != nil && row.ExpiresAt != nil && isExpired(*row.ExpiresAt) {
		return "", "", errors.Unauthorized("API key expired")
	}
	_ = s.repo.TouchLastUsed(id)
	return userID, scope, nil
}

// generateKey returns fin_live_<base64url(32 bytes)>, its SHA-256 hex hash and
// the 14-character display prefix.
func generateKey() (key, hash, prefix string) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		panic("crypto/rand unavailable: " + err.Error())
	}
	key = keyPrefix + base64.URLEncoding.EncodeToString(bytes)
	return key, hashKey(key), key[:14]
}

func hashKey(key string) string {
	digest := sha256.Sum256([]byte(key))
	return hex.EncodeToString(digest[:])
}

// isExpired compares stored go_ts strings lexicographically (same layout).
func isExpired(expiresAt string) bool {
	return expiresAt < tim.NowGoTS()
}
