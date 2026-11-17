// Package sqlite is the SQLite implementation of repo.UserRepo.
package sqlite

import (
	"database/sql"

	"github.com/google/uuid"
	"github.com/mohan9182/financer/internal/repo"
)

type UserRepo struct {
	db *sql.DB
}

func NewUserRepo(db *sql.DB) *UserRepo {
	return &UserRepo{db: db}
}

func (r *UserRepo) Create(email, passwordHash, name string) (string, error) {
	id := uuid.New().String()
	_, err := r.db.Exec(
		"INSERT INTO users (id, email, password_hash, name, created_at) VALUES (?, ?, ?, ?, datetime('now'))",
		id, email, passwordHash, name,
	)
	return id, err
}

func (r *UserRepo) ByEmail(email string) (*repo.UserRow, error) {
	return r.scanOne(r.db.QueryRow(
		"SELECT id, email, name, password_hash, token_version, avatar_url FROM users WHERE email = ?", email,
	))
}

func (r *UserRepo) ByID(id string) (*repo.UserRow, error) {
	return r.scanOne(r.db.QueryRow(
		"SELECT id, email, name, password_hash, token_version, avatar_url FROM users WHERE id = ?", id,
	))
}

func (r *UserRepo) UpdateProfile(id, name, email, avatarURL string) error {
	_, err := r.db.Exec(
		"UPDATE users SET name = COALESCE(NULLIF(?, ''), name), email = COALESCE(NULLIF(?, ''), email), avatar_url = COALESCE(NULLIF(?, ''), avatar_url) WHERE id = ?",
		name, email, avatarURL, id,
	)
	return err
}

func (r *UserRepo) ChangePassword(id, passwordHash string) (int64, error) {
	if _, err := r.db.Exec(
		"UPDATE users SET password_hash = ?, token_version = token_version + 1 WHERE id = ?",
		passwordHash, id,
	); err != nil {
		return 0, err
	}
	var ver int64
	if err := r.db.QueryRow("SELECT token_version FROM users WHERE id = ?", id).Scan(&ver); err != nil {
		return 0, err
	}
	return ver, nil
}

func (r *UserRepo) LogoutAll(id string) error {
	_, err := r.db.Exec("UPDATE users SET token_version = token_version + 1 WHERE id = ?", id)
	return err
}

func (r *UserRepo) scanOne(row *sql.Row) (*repo.UserRow, error) {
	var u repo.UserRow
	var name, pwHash, avatar sql.NullString
	if err := row.Scan(&u.ID, &u.Email, &name, &pwHash, &u.TokenVersion, &avatar); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if name.Valid {
		u.Name = &name.String
	}
	if pwHash.Valid {
		u.PasswordHash = &pwHash.String
	}
	if avatar.Valid {
		u.AvatarURL = &avatar.String
	}
	return &u, nil
}
