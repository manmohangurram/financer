package keyservice

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mohan9182/financer/internal/db"
	"github.com/mohan9182/financer/internal/repo/sqlite"
	tim "github.com/mohan9182/financer/internal/time"
)

func setup(t *testing.T) (*Service, string, func()) {
	t.Helper()
	dbs, err := db.Open(filepath.Join(t.TempDir(), "financer.db"))
	if err != nil {
		t.Fatalf("opening test db: %v", err)
	}
	users := sqlite.NewUserRepo(dbs.Write)
	uid, err := users.Create("k@test.com", "hash", "Keys")
	if err != nil {
		t.Fatal(err)
	}
	return New(sqlite.NewUserKeyRepo(dbs.Write)), uid, func() {
		dbs.Write.Close()
		dbs.Read.Close()
	}
}

func TestCreateShowsKeyOnceAndListsMasked(t *testing.T) {
	svc, uid, cleanup := setup(t)
	defer cleanup()

	created, err := svc.Create(uid, "laptop", 30, "read_write")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(created.Key, "fin_live_") {
		t.Fatalf("key = %q", created.Key)
	}
	if created.KeyPrefix != created.Key[:14] {
		t.Fatalf("prefix = %q, key = %q", created.KeyPrefix, created.Key)
	}
	if created.Scope != "read_write" || created.ExpiresAt == nil {
		t.Fatalf("created = %+v", created)
	}

	keys, err := svc.List(uid)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 {
		t.Fatalf("keys = %d", len(keys))
	}
	if keys[0].KeyPrefix != created.KeyPrefix || keys[0].Scope != "read_write" {
		t.Fatalf("listed = %+v", keys[0])
	}
	// The plaintext never appears in the list payload.
	if strings.Contains(keys[0].KeyPrefix, created.Key[14:]) {
		t.Fatal("list leaked the secret")
	}
}

func TestScopeDefaultsToReadAndNoExpiry(t *testing.T) {
	svc, uid, cleanup := setup(t)
	defer cleanup()

	created, err := svc.Create(uid, "", 0, "bogus")
	if err != nil {
		t.Fatal(err)
	}
	if created.Scope != "read" {
		t.Fatalf("scope = %q", created.Scope)
	}
	if created.ExpiresAt != nil {
		t.Fatalf("expiresAt = %v", *created.ExpiresAt)
	}
}

func TestAuthenticate(t *testing.T) {
	svc, uid, cleanup := setup(t)
	defer cleanup()

	created, err := svc.Create(uid, "mcp", 0, "read")
	if err != nil {
		t.Fatal(err)
	}
	gotUID, scope, err := svc.Authenticate(created.Key)
	if err != nil {
		t.Fatal(err)
	}
	if gotUID != uid || scope != "read" {
		t.Fatalf("authenticate = %q %q", gotUID, scope)
	}
	if _, _, err := svc.Authenticate("fin_live_wrong"); err == nil {
		t.Fatal("unknown key must be rejected")
	}

	// last_used_at is recorded.
	keys, _ := svc.List(uid)
	if keys[0].LastUsedAt == nil {
		t.Fatal("lastUsedAt not recorded")
	}
}

func TestAuthenticateRejectsExpired(t *testing.T) {
	svc, uid, cleanup := setup(t)
	defer cleanup()

	plain := "fin_live_expiredkey"
	past := tim.FromUTC(tim.NowUTC().AddDate(0, 0, -1))
	if _, err := svc.repo.Create(uid, "old", hashKey(plain), "fin_live_expire", &past, "read"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Authenticate(plain); err == nil {
		t.Fatal("expired key must be rejected")
	}
}

func TestMaxKeysPerUser(t *testing.T) {
	svc, uid, cleanup := setup(t)
	defer cleanup()
	for i := 0; i < maxKeysPerUser; i++ {
		if _, err := svc.Create(uid, "k", 0, "read"); err != nil {
			t.Fatalf("create %d: %v", i, err)
		}
	}
	if _, err := svc.Create(uid, "overflow", 0, "read"); err == nil {
		t.Fatal("must refuse the 51st key")
	}
}

func TestDelete(t *testing.T) {
	svc, uid, cleanup := setup(t)
	defer cleanup()
	created, err := svc.Create(uid, "x", 0, "read")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(uid, created.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(uid, created.ID); err == nil {
		t.Fatal("second delete must be not-found")
	}
}
