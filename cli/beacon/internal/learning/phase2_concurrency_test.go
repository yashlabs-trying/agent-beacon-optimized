package learning

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/asymptote-labs/agent-beacon/pkg/asymptoteobserve"
)

// phase2_concurrency_test.go drives the store from many goroutines against a
// disposable database and reports throughput, error rate, busy errors, latency
// percentiles, goroutine growth and WAL size. Concurrency and duration are
// configurable so a short CI run and a longer soak use the same harness.
//
// Usage:
//
//	BEACON_PHASE2_STRESS=1 BEACON_PHASE2_CONCURRENCY=16 BEACON_PHASE2_SECONDS=15 \
//	  go test ./internal/learning -run TestPhase2ConcurrencyStress -v
//
// The workload mixes reads (lexical + hybrid + ANN), writes, supersession under
// read, and index rebuilds (triggered by dropping the derived index while
// readers run), so cache invalidation and rebuild-under-read are exercised.

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(p / 100 * float64(len(sorted)-1))
	return sorted[idx]
}

type latencyRecorder struct {
	mu    sync.Mutex
	spans []float64 // microseconds
}

func (r *latencyRecorder) record(start time.Time) {
	d := float64(time.Since(start).Microseconds())
	r.mu.Lock()
	r.spans = append(r.spans, d)
	r.mu.Unlock()
}

func (r *latencyRecorder) stats() (p50, p95, p99, mean float64, n int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.spans) == 0 {
		return 0, 0, 0, 0, 0
	}
	s := append([]float64(nil), r.spans...)
	sort.Float64s(s)
	var sum float64
	for _, v := range s {
		sum += v
	}
	return percentile(s, 50), percentile(s, 95), percentile(s, 99), sum / float64(len(s)), len(s)
}

func TestPhase2ConcurrencyStress(t *testing.T) {
	if os.Getenv("BEACON_PHASE2_STRESS") != "1" {
		t.Skip("set BEACON_PHASE2_STRESS=1 to run the concurrency stress harness")
	}
	concurrency := envInt("BEACON_PHASE2_CONCURRENCY", 8)
	seconds := envInt("BEACON_PHASE2_SECONDS", 10)
	if concurrency < 1 {
		concurrency = 1
	}

	dbPath := filepath.Join(t.TempDir(), "memory.db")
	store := OpenSemantic(dbPath, NewHashedEmbedder())
	project := asymptoteobserve.LearningProjectV1{ID: "proj-stress", Path: "/repos/stress"}

	// Seed a base corpus so reads have data to scan.
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
		reads, writes, supersedes, errorsCount, busyCount, staleCount atomic.Int64
		readLat, writeLat                                             latencyRecorder
	)
	deadline := time.Now().Add(time.Duration(seconds) * time.Second)
	goroutinesBefore := runtime.NumGoroutine()

	// One goroutine periodically drops the derived index so readers race a
	// rebuild; this is the rebuild-during-writes / cache-invalidation case.
	stopChurn := make(chan struct{})
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

	var wg sync.WaitGroup
	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			i := 0
			for time.Now().Before(deadline) {
				i++
				switch {
				case i%5 == 0: // write
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
				case i%17 == 0: // supersession under read
					start := time.Now()
					old := fmt.Sprintf("seed-%04d", i%300)
					newID := fmt.Sprintf("w-%02d-%06d", worker, i)
					_, _, _ = store.GetMemory(old)
					if err := store.PutMemory(asymptoteobserve.LearningMemoryV1{
						ID: old, CandidateID: fmt.Sprintf("c-seed-%04d", i%300),
						Kind: asymptoteobserve.LearningMemoryKindConvention, Title: "seed",
						Body: "superseded seed", Project: project,
						SupersededBy: newID, Evidence: []asymptoteobserve.LearningEvidenceV1{{TraceID: "tr-s"}},
					}); err == nil {
						supersedes.Add(1)
					}
					writeLat.record(start)
				default: // read
					start := time.Now()
					_, err := store.ListMemories(Query{ProjectID: "proj-stress", Q: "cache retry", Limit: 10})
					readLat.record(start)
					if err != nil {
						errorsCount.Add(1)
					} else {
						reads.Add(1)
					}
					if i%3 == 0 {
						_, _ = store.SearchHybrid(Query{ProjectID: "proj-stress", Q: "cache retry"}, HybridOptions{Limit: 5, CandidateK: 20})
					}
				}
			}
		}(w)
	}
	wg.Wait()
	close(stopChurn)
	time.Sleep(100 * time.Millisecond)

	rp50, rp95, rp99, rmean, rn := readLat.stats()
	wp50, wp95, wp99, wmean, wn := writeLat.stats()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	walSize := fileSize(dbPath + "-wal")
	goroutinesAfter := runtime.NumGoroutine()

	t.Logf("STRESS concurrency=%d duration_s=%d reads=%d writes=%d supersedes=%d errors=%d busy=%d stale=%d",
		concurrency, seconds, reads.Load(), writes.Load(), supersedes.Load(), errorsCount.Load(), busyCount.Load(), staleCount.Load())
	t.Logf("STRESS read_latency_us p50=%.0f p95=%.0f p99=%.0f mean=%.0f n=%d", rp50, rp95, rp99, rmean, rn)
	t.Logf("STRESS write_latency_us p50=%.0f p95=%.0f p99=%.0f mean=%.0f n=%d", wp50, wp95, wp99, wmean, wn)
	t.Logf("STRESS goroutines before=%d after=%d heap_alloc_mb=%.1f heap_sys_mb=%.1f num_gc=%d wal_bytes=%d",
		goroutinesBefore, goroutinesAfter, float64(m.HeapAlloc)/(1<<20), float64(m.Sys)/(1<<20), m.NumGC, walSize)

	// Post-conditions: no non-busy errors, database intact, no goroutine leak.
	// Busy errors are only tolerated under the race detector, which slows the
	// process enough that a write can exceed busy_timeout; in a normal build the
	// harness asserts zero errors of any kind.
	nonBusy := errorsCount.Load() - busyCount.Load()
	if nonBusy > 0 {
		t.Errorf("stress produced %d non-busy errors", nonBusy)
	}
	if !raceEnabled && busyCount.Load() > 0 {
		t.Errorf("stress produced %d busy errors (busy_timeout exceeded)", busyCount.Load())
	}
	if raceEnabled && busyCount.Load() > 0 {
		t.Logf("NOTE: %d SQLITE_BUSY under race instrumentation (busy_timeout exceeded); not a race", busyCount.Load())
	}
	if goroutinesAfter > goroutinesBefore+4 {
		t.Errorf("possible goroutine leak: before=%d after=%d", goroutinesBefore, goroutinesAfter)
	}
	db := openRaw(t, dbPath)
	defer db.Close()
	integrityCheck(t, db)
}

// TestPhase2CacheInvalidationUnderWrites checks that a cached search result is
// never served after a write commits: the durable generation advances and the
// next lookup misses.
func TestPhase2CacheInvalidationUnderWrites(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "memory.db")
	store := OpenSemantic(dbPath, NewHashedEmbedder())
	project := asymptoteobserve.LearningProjectV1{ID: "proj-cache", Path: "/repos/cache"}

	put := func(id string) {
		if err := store.PutMemory(asymptoteobserve.LearningMemoryV1{
			ID: id, CandidateID: "c-" + id, Kind: asymptoteobserve.LearningMemoryKindConvention,
			Title: "note", Body: "invalidation-marker memory", Project: project,
			Evidence: []asymptoteobserve.LearningEvidenceV1{{TraceID: "tr-" + id}},
		}); err != nil {
			t.Fatal(err)
		}
	}
	put("m1")
	first, err := store.ListMemories(Query{ProjectID: "proj-cache", Q: "invalidation-marker", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 {
		t.Fatalf("first read = %d, want 1", len(first))
	}
	put("m2")
	second, err := store.ListMemories(Query{ProjectID: "proj-cache", Q: "invalidation-marker", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 2 {
		t.Fatalf("stale cache served: got %d, want 2", len(second))
	}
}
