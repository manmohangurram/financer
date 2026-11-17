package accountservice

import (
	"path/filepath"
	"testing"

	"github.com/mohan9182/financer/internal/db"
	"github.com/mohan9182/financer/internal/repo/sqlite"
)

func TestListPageCursor(t *testing.T) {
	dbs, err := db.Open(filepath.Join(t.TempDir(), "financer.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer dbs.Write.Close()
	defer dbs.Read.Close()
	uid, err := sqlite.NewUserRepo(dbs.Write).Create("pages@test.com", "hash", "Pages")
	if err != nil {
		t.Fatal(err)
	}
	svc := New(sqlite.NewAccountRepo(dbs.Write))
	for _, name := range []string{"A", "B", "C"} {
		if _, err := svc.Create(uid, name, name, "SAVINGS", "1234"); err != nil {
			t.Fatal(err)
		}
	}

	page1, next, err := svc.ListPage(uid, 2, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(page1) != 2 || next == "" {
		t.Fatalf("page1=%d next=%q", len(page1), next)
	}
	page2, next2, err := svc.ListPage(uid, 2, next)
	if err != nil {
		t.Fatal(err)
	}
	if len(page2) != 1 || next2 != "" {
		t.Fatalf("page2=%d next=%q", len(page2), next2)
	}
	if page1[0].ID == page2[0].ID || page1[1].ID == page2[0].ID {
		t.Fatal("pages overlap")
	}
	if _, _, err := svc.ListPage(uid, 2, "not-a-token"); err == nil {
		t.Fatal("invalid token must fail")
	}
}
