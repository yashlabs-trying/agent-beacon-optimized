package learning

// Phase 3C diagnostic probes for the remaining 16-client write tail.
//
// TestPhase3CCheckpointProbe isolates three contributors on a shared pool:
// transaction weight (FTS trigram upsert), concurrent readers, and WAL
// auto-checkpoint stalls. TestPhase3CWritePhaseProfile times the phases of the
// real PutMemory write path under 16-writer contention.

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
	"github.com/asymptote-labs/agent-beacon/pkg/asymptoteobserve"
)

func p3cProbeDB(t *testing.T, dbPath, checkpoint string) *sql.DB {
	t.Helper()
	pragmas := []string{"busy_timeout(30000)"}
	if checkpoint != "" {
		pragmas = append(pragmas, "wal_autocheckpoint("+checkpoint+")")
	}
	db, err := sql.Open("sqlite", sqliteuri.File(dbPath, url.Values{
		"_pragma":       pragmas,
		"_busy_timeout": {"30000"},
		"_txlock":       {"immediate"},
	}))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(64)
	db.SetMaxIdleConns(64)
	if _, err := db.Exec(`PRAGMA journal_mode=WAL`); err != nil {
		t.Fatal(err)
	}
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS memories (id TEXT PRIMARY KEY, project_id TEXT NOT NULL, body TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS meta (k TEXT PRIMARY KEY, v INTEGER NOT NULL)`,
		`INSERT INTO meta (k, v) VALUES ('gen', 0) ON CONFLICT(k) DO NOTHING`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS memories_fts USING fts5(memory_id UNINDEXED, hay, tokenize='trigram case_sensitive 1')`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func TestPhase3CCheckpointProbe(t *testing.T) {
	if os.Getenv("BEACON_PHASE3C_PROBE") != "1" {
		t.Skip("set BEACON_PHASE3C_PROBE=1")
	}
	concurrency := envInt("BEACON_PHASE3C_PROBE_CONCURRENCY", 16)
	seconds := envInt("BEACON_PHASE3C_PROBE_SECONDS", 5)
	scenarios := []struct {
		name       string
		readers    int
		checkpoint string
	}{
		{"no-readers/default-checkpoint", 0, ""},
		{"readers/default-checkpoint", 16, ""},
		{"readers/checkpoint-off", 16, "0"},
		{"readers/checkpoint-10000", 16, "10000"},
	}
	for _, sc := range scenarios {
		dbPath := filepath.Join(t.TempDir(), "probe.db")
		db := p3cProbeDB(t, dbPath, sc.checkpoint)
		// seed
		for i := 0; i < 300; i++ {
			db.Exec(`INSERT INTO memories (id, project_id, body) VALUES (?, 'p', 'seed cache retry')`, fmt.Sprintf("seed-%04d", i))
		}
		var writeLat, readLat latencyRecorder
		deadline := time.Now().Add(time.Duration(seconds) * time.Second)
		var wg sync.WaitGroup
		for w := 0; w < concurrency; w++ {
			wg.Add(1)
			go func(w int) {
				defer wg.Done()
				i := 0
				for time.Now().Before(deadline) {
					i++
					start := time.Now()
					id := fmt.Sprintf("w-%02d-%06d", w, i)
					body := "stress write " + id + " about cache retry"
					tx, err := db.Begin()
					if err != nil {
						continue
					}
					_, _ = tx.Exec(`INSERT INTO memories (id, project_id, body) VALUES (?, 'p', ?) ON CONFLICT(id) DO UPDATE SET body = excluded.body`, id, body)
					_, _ = tx.Exec(`DELETE FROM memories_fts WHERE memory_id = ?`, id)
					_, _ = tx.Exec(`INSERT INTO memories_fts (memory_id, hay) VALUES (?, ?)`, id, id+"\n"+body)
					_, _ = tx.Exec(`UPDATE meta SET v = v + 1 WHERE k = 'gen'`)
					if err := tx.Commit(); err != nil {
						continue
					}
					writeLat.record(start)
				}
			}(w)
		}
		for r := 0; r < sc.readers; r++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for time.Now().Before(deadline) {
					start := time.Now()
					var n int
					_ = db.QueryRow(`SELECT COUNT(*) FROM memories WHERE project_id = 'p'`).Scan(&n)
					readLat.record(start)
				}
			}()
		}
		wg.Wait()
		wp50, wp95, wp99, _, wn := writeLat.stats()
		_, _, rp99, _, rn := readLat.stats()
		wal := fileSize(dbPath + "-wal")
		fmt.Printf("P3C_PROBE scenario=%s writers=%d readers=%d write_p50=%.0f write_p95=%.0f write_p99=%.0f wn=%d read_p99=%.0f rn=%d wal_bytes=%d\n",
			sc.name, concurrency, sc.readers, wp50, wp95, wp99, wn, rp99, rn, wal)
		db.Close()
	}
}

// TestPhase3CWritePhaseProfile times the phases of the real write path in a
// single-writer loop (so consecutive-stage deltas are exact), using the
// writeCheckpoint test seam.
func TestPhase3CWritePhaseProfile(t *testing.T) {
	if os.Getenv("BEACON_PHASE3C_PROBE") != "1" {
		t.Skip("set BEACON_PHASE3C_PROBE=1")
	}
	iters := envInt("BEACON_PHASE3C_PROBE_ITERS", 200)
	dbPath := filepath.Join(t.TempDir(), "memory.db")
	store := OpenSemantic(dbPath, NewHashedEmbedder())
	project := asymptoteobserve.LearningProjectV1{ID: "proj-p"}
	for i := 0; i < 300; i++ {
		store.PutMemory(asymptoteobserve.LearningMemoryV1{
			ID: fmt.Sprintf("seed-%04d", i), CandidateID: fmt.Sprintf("c-%04d", i),
			Kind: asymptoteobserve.LearningMemoryKindConvention, Title: "seed",
			Body: "seed memory about cache retry", Project: project,
			Evidence: []asymptoteobserve.LearningEvidenceV1{{TraceID: "t"}},
		})
	}

	phases := map[string]*latencyRecorder{}
	rec := func(name string) *latencyRecorder {
		if phases[name] == nil {
			phases[name] = &latencyRecorder{}
		}
		return phases[name]
	}
	var last time.Time
	store.writeCheckpoint = func(stage string) {
		now := time.Now()
		if !last.IsZero() {
			rec(stage).record(last)
		}
		last = now
	}

	var total latencyRecorder
	for i := 0; i < iters; i++ {
		id := fmt.Sprintf("w-%06d", i)
		start := time.Now()
		_ = store.PutMemory(asymptoteobserve.LearningMemoryV1{
			ID: id, CandidateID: "c-" + id, Kind: asymptoteobserve.LearningMemoryKindGotcha,
			Title: "stress", Body: "stress write " + id + " about cache retry", Project: project,
			Evidence: []asymptoteobserve.LearningEvidenceV1{{TraceID: "t"}},
		})
		total.record(start)
	}
	tp50, tp95, tp99, _, tn := total.stats()
	fmt.Printf("P3C_PHASE total p50=%.0f p95=%.0f p99=%.0f n=%d\n", tp50, tp95, tp99, tn)
	for _, name := range []string{"after-begin", "after-authoritative", "after-index", "after-generation", "before-commit", "after-commit"} {
		r := phases[name]
		if r == nil {
			continue
		}
		p50, p95, p99, _, n := r.stats()
		fmt.Printf("P3C_PHASE phase=%s p50=%.0f p95=%.0f p99=%.0f n=%d\n", name, p50, p95, p99, n)
	}
	store.Close()
}
