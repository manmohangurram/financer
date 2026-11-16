package userservice

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mohan9182/financer/internal/auth"
	"github.com/mohan9182/financer/internal/db"
	"github.com/mohan9182/financer/internal/repo"
	"github.com/mohan9182/financer/internal/repo/sqlite"
)

func newTestService(t *testing.T) (*Service, string, repo.UserRepo, func()) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "financer.db")
	dbs, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("opening test db: %v", err)
	}
	avatarDir := filepath.Join(dir, "avatars")
	repo := sqlite.NewUserRepo(dbs.Write)
	j := auth.New("secret")
	cleanup := func() {
		dbs.Write.Close()
		dbs.Read.Close()
		os.Remove(dbPath)
		os.RemoveAll(avatarDir)
	}
	return New(repo, j, avatarDir), avatarDir, repo, cleanup
}

func TestProfileCRUD(t *testing.T) {
	s, avatarDir, repo, cleanup := newTestService(t)
	defer cleanup()

	if _, err := s.Profile("missing"); err == nil {
		t.Fatal("expected not-found for missing user")
	}

	// Seed a user directly through the repo.
	id, err := repo.Create("a@b.c", "hash", "Alice")
	if err != nil {
		t.Fatal(err)
	}

	p, err := s.Profile(id)
	if err != nil {
		t.Fatal(err)
	}
	if p.UserID != id || p.Email != "a@b.c" || p.Name != "Alice" {
		t.Fatalf("profile = %+v", p)
	}

	// Update profile.
	png := []byte("\x89PNG\r\n\x1a\nxxxx")
	up, err := s.SaveAvatar(id, png)
	if err != nil {
		t.Fatal(err)
	}
	if up.AvatarURL == "" {
		t.Fatal("avatar url empty after upload")
	}
	data, err := os.ReadFile(filepath.Join(avatarDir, id+".png"))
	if err != nil || string(data) != string(png) {
		t.Fatalf("avatar file not written: %v", err)
	}

	// Reject oversized/invalid avatars.
	if _, err := s.SaveAvatar(id, []byte("not an image")); err == nil {
		t.Fatal("expected error for invalid image")
	}
}

func TestAvatarSniffing(t *testing.T) {
	cases := []struct {
		data []byte
		ext  string
		ok   bool
	}{
		{[]byte("\x89PNG\r\n\x1a\nxxxx"), ".png", true},
		{[]byte{0xff, 0xd8, 0xff, 0xe0}, ".jpg", true},
		{[]byte("RIFF\x00\x00\x00\x00WEBPxxxx"), ".webp", true},
		{[]byte("not an image"), "", false},
		{nil, "", false},
	}
	for _, c := range cases {
		ext, ok := sniffAvatar(c.data)
		if ok != c.ok || ext != c.ext {
			t.Errorf("sniffAvatar(%q) = (%q, %v), want (%q, %v)", c.data, ext, ok, c.ext, c.ok)
		}
	}
}
