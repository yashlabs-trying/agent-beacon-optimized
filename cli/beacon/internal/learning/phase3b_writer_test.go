package learning

// Phase 3B writer-latency profiling harness.
//
// It reproduces the Phase 3A mixed workload and adds knobs to separate
// steady-state latency from index-rebuild/maintenance spikes:
//
//	BEACON_PHASE3_WRITER=1
//	BEACON_PHASE3_W_CONCURRENCY=16
//	BEACON_PHASE3_W_SECONDS=10
//	BEACON_PHASE3_W_REPS=2
//	BEACON_PHASE3_W_CHURN=1|0   (drop the derived index every 2s)
//	BEACON_PHASE3_W_MODE=mixed|write
//
// It reports per-repetition read/write percentiles, errors, busy, stale, WAL,
// peak RSS, and the average cost of opening+closing a store connection.

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/asymptote-labs/agent-beacon/pkg/asymptoteobserve"
)

func TestPhase3BWriterLatency(t *testing.T) {
	if os.Getenv("BEACON_PHASE3_WRITER") != "1" {
		t.Skip("set BEACON_PHASE3_WRITER=1 to run the writer-latency harness")
	}
	concurrency := envInt("BEACON_PHASE3_W_CONCURRENCY", 16)
	seconds := envInt("BEACON_PHASE3_W_SECONDS", 10)
	reps := envInt("BEACON_PHASE3_W_REPS", 2)
	churn := os.Getenv("BEACON_PHASE3_W_CHURN") == "1"
	mode := os.Getenv("BEACON_PHASE3_W_MODE")
	if mode == "" {
		mode = "mixed"
	}
	if concurrency < 1 {
		concurrency = 1
	}

	for rep := 1; rep <= reps; rep++ {
		dbPath := filepath.Join(t.TempDir(), "memory.db")
		store := OpenSemantic(dbPath, NewHashedEmbedder())
		project := asymptoteobserve.LearningProjectV1{ID: "proj-w", Path: "/repos/w"}
		for i := 0; i < 300; i++ {
			if err := store.PutMemory(asymptoteobserve.LearningMemoryV1{
				ID: fmt.Sprintf("seed-%04d", i), CandidateID: fmt.Sprintf("c-seed-%04d", i),
				Kind: asymptoteobserve.LearningMemoryKindConvention, Title: "seed",
				Body: fmt.Sprintf("seed memory %d about the cache and retry behaviour", i), Project: project,
				Evidence: []asymptoteobserve.LearningEvidenceV1{{TraceID: fmt.Sprintf("tr-%d", i)}},
			}); err != nil {
				t.Fatal(err)
			}
		}

		var (
			reads, writes, errorsCount, busyCount atomic.Int64
			readLat, writeLat                     latencyRecorder
		)
		deadline := time.Now().Add(time.Duration(seconds) * time.Second)
		goroutinesBefore := runtime.NumGoroutine()

		stopChurn := make(chan struct{})
		if churn {
			go func() {
				for {
					select {
					case <-stopChurn:
						return
					case <-time.After(2 * time.Second):
						db, err := store.db()
						if err != nil {
							continue
						}
						_, _ = db.Exec(`UPDATE memory_index_meta SET value = '0' WHERE key = 'index_ready'`)
						schemaInitialized.Delete(dbPath)
						indexReadyCache.Delete(dbPath)
					}
				}
			}()
		}

		var wg sync.WaitGroup
		for w := 0; w < concurrency; w++ {
			wg.Add(1)
			go func(worker int) {
				defer wg.Done()
				i := 0
				for time.Now().Before(deadline) {
					i++
					doWrite := (mode == "write") || (mode != "read" && i%5 == 0)
					if doWrite {
						start := time.Now()
						id := fmt.Sprintf("w-%02d-%06d", worker, i)
						err := store.PutMemory(asymptoteobserve.LearningMemoryV1{
							ID: id, CandidateID: "c-" + id, Kind: asymptoteobserve.LearningMemoryKindGotcha,
							Title: "stress", Body: fmt.Sprintf("stress write %s about cache retry", id), Project: project,
							Evidence: []asymptoteobserve.LearningEvidenceV1{{TraceID: "tr-" + id}},
						})
						writeLat.record(start)
						if err != nil {
							errorsCount.Add(1)
							if strings.Contains(strings.ToLower(err.Error()), "busy") || strings.Contains(strings.ToLower(err.Error()), "locked") {
								busyCount.Add(1)
							}
						} else {
							writes.Add(1)
						}
						continue
					}
					if mode == "write" {
						continue
					}
					q := "cache retry"
					if mode == "select" {
						q = fmt.Sprintf("seed-%04d", i%300)
					}
					start := time.Now()
					_, err := store.ListMemories(Query{ProjectID: "proj-w", Q: q, Limit: 10})
					readLat.record(start)
					if err != nil {
						errorsCount.Add(1)
					} else {
						reads.Add(1)
					}
				}
			}(w)
		}
		wg.Wait()
		if churn {
			close(stopChurn)
		}
		time.Sleep(50 * time.Millisecond)

		rp50, rp95, rp99, _, rn := readLat.stats()
		wp50, wp95, wp99, wmean, wn := writeLat.stats()
		fmt.Printf("P3B_WRITER rep=%d mode=%s churn=%v concurrency=%d duration_s=%d reads=%d writes=%d errors=%d busy=%d read_p50=%.0f read_p95=%.0f read_p99=%.0f write_p50=%.0f write_p95=%.0f write_p99=%.0f write_mean=%.0f rn=%d wn=%d goroutines=%d->%d wal_bytes=%d peak_rss_mb=%d\n",
			rep, mode, churn, concurrency, seconds, reads.Load(), writes.Load(), errorsCount.Load(), busyCount.Load(),
			rp50, rp95, rp99, wp50, wp95, wp99, wmean, rn, wn, goroutinesBefore, runtime.NumGoroutine(), fileSize(dbPath+"-wal"), peakRSSKB()/1024)

		nonBusy := errorsCount.Load() - busyCount.Load()
		if nonBusy > 0 {
			t.Errorf("rep %d produced %d non-busy errors", rep, nonBusy)
		}
		db := openRaw(t, dbPath)
		integrityCheck(t, db)
		db.Close()
		_ = store.Close()
	}
}

// TestPhase3BConnectionCost measures the cost of the per-operation open/close
func TestPhase3BConnectionCost(t *testing.T) {
	if os.Getenv("BEACON_PHASE3_WRITER") != "1" {
		t.Skip("set BEACON_PHASE3_WRITER=1")
	}
	dbPath := filepath.Join(t.TempDir(), "memory.db")
	store := OpenSemantic(dbPath, NewHashedEmbedder())
	store.PutMemory(asymptoteobserve.LearningMemoryV1{
		ID: "m1", CandidateID: "c1", Kind: asymptoteobserve.LearningMemoryKindConvention,
		Title: "t", Body: "b", Project: asymptoteobserve.LearningProjectV1{ID: "p"},
		Evidence: []asymptoteobserve.LearningEvidenceV1{{TraceID: "t"}},
	})
	const n = 2000
	start := time.Now()
	for i := 0; i < n; i++ {
		if _, err := store.db(); err != nil {
			t.Fatal(err)
		}
	}
	fmt.Printf("P3B_CONN pooled_reuse n=%d per_us=%.1f\n", n, float64(time.Since(start).Microseconds())/float64(n))
}

// TestPhase3BConnectionMaterialised compares, under concurrent goroutines, the
// cost of the per-operation open+query+close pattern against a single shared
// pool. It isolates whether physical SQLite connection churn is the source of
// the mixed-workload write tail.
func TestPhase3BConnectionMaterialised(t *testing.T) {
	if os.Getenv("BEACON_PHASE3_WRITER") != "1" {
		t.Skip("set BEACON_PHASE3_WRITER=1")
	}
	concurrency := envInt("BEACON_PHASE3_W_CONCURRENCY", 16)
	seconds := envInt("BEACON_PHASE3_W_SECONDS", 5)
	dbPath := filepath.Join(t.TempDir(), "memory.db")
	store := OpenSemantic(dbPath, NewHashedEmbedder())
	project := asymptoteobserve.LearningProjectV1{ID: "proj-c"}
	for i := 0; i < 300; i++ {
		store.PutMemory(asymptoteobserve.LearningMemoryV1{
			ID: fmt.Sprintf("seed-%04d", i), CandidateID: fmt.Sprintf("c-%04d", i),
			Kind: asymptoteobserve.LearningMemoryKindConvention, Title: "seed",
			Body: "seed memory about cache retry", Project: project,
			Evidence: []asymptoteobserve.LearningEvidenceV1{{TraceID: "t"}},
		})
	}
	run := func(perOp bool) (p50, p95, p99 float64, n int) {
		var lat latencyRecorder
		deadline := time.Now().Add(time.Duration(seconds) * time.Second)
		var wg sync.WaitGroup
		for w := 0; w < concurrency; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for time.Now().Before(deadline) {
					start := time.Now()
					var db *sql.DB
					if perOp {
						d, err := sql.Open("sqlite", dbPath)
						if err != nil {
							continue
						}
						db = d
					} else {
						d, err := store.db()
						if err != nil {
							continue
						}
						db = d
					}
					var c int
					_ = db.QueryRow(`SELECT COUNT(*) FROM memories WHERE project_id = ?`, "proj-c").Scan(&c)
					if perOp {
						_ = db.Close()
					}
					lat.record(start)
				}
			}()
		}
		wg.Wait()
		p50, p95, p99, _, n = lat.stats()
		return
	}
	p50, p95, p99, n := run(true)
	fmt.Printf("P3B_CONN perop_raw concurrency=%d p50=%.0f p95=%.0f p99=%.0f n=%d\n", concurrency, p50, p95, p99, n)
	p50, p95, p99, n = run(false)
	fmt.Printf("P3B_CONN store_pooled concurrency=%d p50=%.0f p95=%.0f p99=%.0f n=%d\n", concurrency, p50, p95, p99, n)
}
