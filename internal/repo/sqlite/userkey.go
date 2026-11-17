package sqlite

import (
	"database/sql"

	"github.com/google/uuid"

	"github.com/mohan9182/financer/internal/repo"
	tim "github.com/mohan9182/financer/internal/time"
)

// UserKeyRepo implements repo.UserKeyRepo over SQLite.
type UserKeyRepo struct {
	db *sql.DB
}

func NewUserKeyRepo(db *sql.DB) *UserKeyRepo {
	return &UserKeyRepo{db: db}
}

func (r *UserKeyRepo) Create(userID, name, keyHash, keyPrefix string, expiresAt *string, scope string) (string, error) {
	id := uuid.New().String()
	_, err := r.db.Exec(
		`INSERT INTO user_keys (id, user_id, name, key_hash, key_prefix, created_at, expires_at, last_used_at, scope)
		 VALUES (?, ?, ?, ?, ?, ?, ?, NULL, ?)`,
		id, userID, name, keyHash, keyPrefix, tim.NowGoTS(), expiresAt, scope,
	)
	if err != nil {
		return "", err
	}
	return id, nil
}

const userKeyColumns = "id, name, key_prefix, created_at, expires_at, last_used_at, scope"

func (r *UserKeyRepo) List(userID string) ([]*repo.UserKeyRow, error) {
	rows, err := r.db.Query(
		"SELECT "+userKeyColumns+" FROM user_keys WHERE user_id = ? ORDER BY created_at DESC", userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*repo.UserKeyRow
	for rows.Next() {
		k, err := scanUserKey(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

func (r *UserKeyRepo) Count(userID string) (int64, error) {
	var n int64
	err := r.db.QueryRow("SELECT COUNT(*) FROM user_keys WHERE user_id = ?", userID).Scan(&n)
	return n, err
}

func (r *UserKeyRepo) GetByID(userID, id string) (*repo.UserKeyRow, error) {
	rows, err := r.db.Query(
		"SELECT "+userKeyColumns+" FROM user_keys WHERE id = ? AND user_id = ?", id, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, rows.Err()
	}
	return scanUserKey(rows)
}

func (r *UserKeyRepo) Delete(userID, id string) (bool, error) {
	res, err := r.db.Exec("DELETE FROM user_keys WHERE id = ? AND user_id = ?", id, userID)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func (r *UserKeyRepo) ByKeyHash(keyHash string) (string, string, string, bool, error) {
	var userID, id, scope string
	err := r.db.QueryRow(
		"SELECT user_id, id, scope FROM user_keys WHERE key_hash = ?", keyHash,
	).Scan(&userID, &id, &scope)
	if err == sql.ErrNoRows {
		return "", "", "", false, nil
	}
	if err != nil {
		return "", "", "", false, err
	}
	return userID, id, scope, true, nil
}

func (r *UserKeyRepo) TouchLastUsed(id string) error {
	_, err := r.db.Exec("UPDATE user_keys SET last_used_at = ? WHERE id = ?", tim.NowGoTS(), id)
	return err
}

func scanUserKey(rows *sql.Rows) (*repo.UserKeyRow, error) {
	var k repo.UserKeyRow
	var expiresAt, lastUsedAt sql.NullString
	if err := rows.Scan(&k.ID, &k.Name, &k.KeyPrefix, &k.CreatedAt, &expiresAt, &lastUsedAt, &k.Scope); err != nil {
		return nil, err
	}
	if expiresAt.Valid {
		k.ExpiresAt = &expiresAt.String
	}
	if lastUsedAt.Valid {
		k.LastUsedAt = &lastUsedAt.String
	}
	return &k, nil
}
