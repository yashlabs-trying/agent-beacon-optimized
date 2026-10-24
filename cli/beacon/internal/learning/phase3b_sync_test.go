package learning

// Phase 3B diagnostic: isolate the cost of SQLite fsync (synchronous) from
// connection churn and lock contention. It runs the same concurrent
// BEGIN IMMEDIATE / INSERT / COMMIT pattern on a shared pool at three
// synchronous settings. Diagnostic only; it does not change store behaviour.

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/asymptote-labs/agent-beacon/cli/beacon/internal/sqliteuri"
)

func TestPhase3BSynchronousCost(t *testing.T) {
	if os.Getenv("BEACON_PHASE3_WRITER") != "1" {
		t.Skip("set BEACON_PHASE3_WRITER=1")
	}
	concurrency := envInt("BEACON_PHASE3_W_CONCURRENCY", 16)
	seconds := envInt("BEACON_PHASE3_W_SECONDS", 5)
	for _, mode := range []string{"FULL", "NORMAL", "OFF"} {
		runSynchronousProbe(t, concurrency, seconds, mode)
	}
}

func runSynchronousProbe(t *testing.T, concurrency, seconds int, syncMode string) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "probe.db")
	db, err := sql.Open("sqlite", sqliteuri.File(dbPath, url.Values{
		"_pragma":       {"busy_timeout(30000)", "synchronous(" + syncMode + ")"},
		"_busy_timeout": {"30000"},
		"_txlock":       {"immediate"},
	}))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(concurrency + 2)
	if _, err := db.Exec(`PRAGMA journal_mode=WAL`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS probe (id TEXT PRIMARY KEY, body TEXT)`); err != nil {
		t.Fatal(err)
	}

	var lat latencyRecorder
	deadline := time.Now().Add(time.Duration(seconds) * time.Second)
	var wg sync.WaitGroup
	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			i := 0
			for time.Now().Before(deadline) {
				i++
				start := time.Now()
				tx, err := db.Begin()
				if err != nil {
					continue
				}
				if _, err := tx.Exec(`INSERT INTO probe (id, body) VALUES (?, ?) ON CONFLICT(id) DO UPDATE SET body = excluded.body`,
					fmt.Sprintf("w-%02d-%06d", worker, i), "stress write about cache retry"); err != nil {
					_ = tx.Rollback()
					continue
				}
				if err := tx.Commit(); err != nil {
					continue
				}
				lat.record(start)
			}
		}(w)
	}
	wg.Wait()
	p50, p95, p99, mean, n := lat.stats()
	fmt.Printf("P3B_SYNC mode=%s concurrency=%d p50=%.0f p95=%.0f p99=%.0f mean=%.0f n=%d\n", syncMode, concurrency, p50, p95, p99, mean, n)
}
