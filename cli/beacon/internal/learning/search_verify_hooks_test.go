package learning

import (
	"database/sql"
	"testing"
	"time"
)

// verifyResetCache clears the process result cache so the next query is cold.
// The baseline has no cache, so its copy of this file is a no-op.
func verifyResetCache() {
	resetSearchCacheForTest()
}

// verifyPrepareIndex builds the derived index and returns how long it took. The
// baseline has no derived index, so its copy returns 0.
func verifyPrepareIndex(tb testing.TB, store *Store, db *sql.DB) time.Duration {
	tb.Helper()
	start := time.Now()
	if !store.ensureIndexReady(db) {
		tb.Fatal("derived index did not become ready")
	}
	return time.Since(start)
}
