package learning

import (
	"database/sql"
	"testing"
)

// openRaw opens the store's database directly for integrity and row checks. It
// is platform-neutral, so it lives outside the Unix-only crash-injection file
// that also uses it.
func openRaw(t *testing.T, dbPath string) *sql.DB {
	t.Helper()
	store := Open(dbPath)
	db, err := store.db()
	if err != nil {
		t.Fatalf("reopen %s: %v", dbPath, err)
	}
	return db
}

func integrityCheck(t *testing.T, db *sql.DB) {
	t.Helper()
	var result string
	if err := db.QueryRow(`PRAGMA integrity_check`).Scan(&result); err != nil {
		t.Fatalf("integrity_check: %v", err)
	}
	if result != "ok" {
		t.Fatalf("integrity_check = %q, want ok", result)
	}
}
