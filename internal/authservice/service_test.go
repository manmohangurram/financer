package authservice

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/mohan9182/financer/internal/auth"
	"github.com/mohan9182/financer/internal/db"
	apierrors "github.com/mohan9182/financer/internal/errors"
	"github.com/mohan9182/financer/internal/repo/sqlite"
)

func newTestDB(t *testing.T) (*sql.DB, func()) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "financer.db")
	dbs, err := db.Open(path)
	if err != nil {
		t.Fatalf("opening test db: %v", err)
	}
	cleanup := func() {
		dbs.Write.Close()
		dbs.Read.Close()
		os.Remove(path)
	}
	return dbs.Write, cleanup
}

func TestSignupLoginRefresh(t *testing.T) {
	writeDB, cleanup := newTestDB(t)
	defer cleanup()
	repo := sqlite.NewUserRepo(writeDB)
	j := auth.New("secret")
	s := New(repo, j)

	// Signup.
	out, err := s.Signup("a@b.c", "password", "Alice")
	if err != nil {
		t.Fatal(err)
	}
	if out.UserID == "" || out.AccessToken == "" || out.RefreshToken == "" {
		t.Fatalf("incomplete auth response: %+v", out)
	}

	// Duplicate email -> conflict (already_exists).
	if _, err := s.Signup("a@b.c", "password", "Bob"); err == nil {
		t.Fatal("expected conflict for duplicate email")
	} else if apiErr, ok := err.(*apierrors.Error); !ok || apiErr.Status != 409 || apiErr.Code != "already_exists" {
		t.Fatalf("duplicate signup error = %#v, want 409 already_exists", err)
	}

	// Login.
	out, err = s.Login("a@b.c", "password")
	if err != nil {
		t.Fatal(err)
	}
	if out.UserID == "" {
		t.Fatal("user id empty")
	}

	// Wrong password -> unauthorized.
	if _, err := s.Login("a@b.c", "wrong"); err == nil {
		t.Fatal("expected unauthorized for wrong password")
	}

	// Refresh rotates tokens.
	refreshed, err := s.RefreshToken(out.RefreshToken)
	if err != nil {
		t.Fatal(err)
	}
	if refreshed.UserID != out.UserID {
		t.Fatal("refresh user id mismatch")
	}

	// Bad refresh token.
	if _, err := s.RefreshToken("not-a-token"); err == nil {
		t.Fatal("expected error for bad refresh token")
	}
}

func TestLogoutAllRevokesRefresh(t *testing.T) {
	writeDB, cleanup := newTestDB(t)
	defer cleanup()
	repo := sqlite.NewUserRepo(writeDB)
	j := auth.New("secret")
	s := New(repo, j)

	if _, err := s.Signup("a@b.c", "password", "Alice"); err != nil {
		t.Fatal(err)
	}
	user, err := repo.ByEmail("a@b.c")
	if err != nil || user == nil {
		t.Fatal(err)
	}
	out, err := s.Login("a@b.c", "password")
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.LogoutAll(user.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RefreshToken(out.RefreshToken); err == nil {
		t.Fatal("expected revoked refresh token after logout-all")
	}
}
