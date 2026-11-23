package learning

// P1 tests: batch writes. Correctness, atomic authorization, size bound,
// idempotency, and an ingestion benchmark that emits a CSV when BENCH_OUT_P1 is
// set (used for the before/after measurement).

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/asymptote-labs/agent-beacon/pkg/asymptoteobserve"
)

func p1Mem(i int) asymptoteobserve.LearningMemoryV1 {
	return asymptoteobserve.LearningMemoryV1{
		ID: fmt.Sprintf("m-%d", i), CandidateID: fmt.Sprintf("c-%d", i),
		Kind: asymptoteobserve.LearningMemoryKindConvention, Title: "t",
		Body: "batch body text about workflow and retries",
		Project: asymptoteobserve.LearningProjectV1{ID: "p"},
	}
}

func TestP1BatchCorrectness(t *testing.T) {
	dbPath := filepath.Join(supTempDir(t), "memory.db")
	s := Open(dbPath)
	defer supClose(s)
	var ms []asymptoteobserve.LearningMemoryV1
	for i := 0; i < 250; i++ {
		ms = append(ms, p1Mem(i))
	}
	if err := s.PutMemories(ms); err != nil {
		t.Fatalf("PutMemories: %v", err)
	}
	db, _ := s.db()
	var count int
	_ = db.QueryRow(`SELECT COUNT(*) FROM memories`).Scan(&count)
	if count != 250 {
		t.Fatalf("rows = %d, want 250", count)
	}
	if _, ok, err := s.GetMemory("m-123"); err != nil || !ok {
		t.Fatalf("GetMemory after batch: ok=%v err=%v", ok, err)
	}
}

func TestP1BatchAuthorizationAtomic(t *testing.T) {
	dbPath := filepath.Join(supTempDir(t), "memory.db")
	s := OpenAuthorized(dbPath, NewProjectAllowlist("project-A"))
	defer supClose(s)
	ms := []asymptoteobserve.LearningMemoryV1{
		{ID: "a", CandidateID: "ca", Kind: asymptoteobserve.LearningMemoryKindConvention, Title: "t", Body: "b", Project: asymptoteobserve.LearningProjectV1{ID: "project-A"}},
		{ID: "b", CandidateID: "cb", Kind: asymptoteobserve.LearningMemoryKindConvention, Title: "t", Body: "b", Project: asymptoteobserve.LearningProjectV1{ID: "project-B"}},
	}
	if err := s.PutMemories(ms); err == nil {
		t.Fatal("batch with a foreign project must be rejected")
	}
	db, _ := s.db()
	var count int
	_ = db.QueryRow(`SELECT COUNT(*) FROM memories`).Scan(&count)
	if count != 0 {
		t.Fatalf("batch was not atomic: %d rows written", count)
	}
}

func TestP1BatchSizeBound(t *testing.T) {
	dbPath := filepath.Join(supTempDir(t), "memory.db")
	s := Open(dbPath)
	defer supClose(s)
	ms := make([]asymptoteobserve.LearningMemoryV1, MaxBatchSize+1)
	for i := range ms {
		ms[i] = p1Mem(i)
	}
	if err := s.PutMemories(ms); err == nil {
		t.Fatal("oversized batch must be rejected")
	}
}

func TestP1BatchIdempotentRetry(t *testing.T) {
	dbPath := filepath.Join(supTempDir(t), "memory.db")
	s := Open(dbPath)
	defer supClose(s)
	var ms []asymptoteobserve.LearningMemoryV1
	for i := 0; i < 50; i++ {
		ms = append(ms, p1Mem(i))
	}
	if err := s.PutMemories(ms); err != nil {
		t.Fatal(err)
	}
	if err := s.PutMemories(ms); err != nil {
		t.Fatal(err)
	}
	db, _ := s.db()
	var count int
	_ = db.QueryRow(`SELECT COUNT(*) FROM memories`).Scan(&count)
	if count != 50 {
		t.Fatalf("idempotent retry produced %d rows, want 50", count)
	}
}

func TestP1IngestionBenchmark(t *testing.T) {
	out := os.Getenv("BENCH_OUT_P1")
	if out == "" {
		t.Skip("set BENCH_OUT_P1")
	}
	const n = 10000
	type res struct {
		mode       string
		batch      int
		totalMs    float64
		perRecordU float64
	}
	var results []res

	// single-record ingestion
	{
		s := Open(filepath.Join(supTempDir(t), "single.db"))
		start := time.Now()
		for i := 0; i < n; i++ {
			if err := s.PutMemory(p1Mem(i)); err != nil {
				t.Fatal(err)
			}
		}
		d := time.Since(start)
		results = append(results, res{"single", 1, float64(d.Milliseconds()), float64(d.Microseconds()) / n})
		supClose(s)
	}
	// batched ingestion at several batch sizes
	for _, b := range []int{10, 25, 50, 100} {
		s := Open(filepath.Join(supTempDir(t), fmt.Sprintf("batch%d.db", b)))
		start := time.Now()
		for i := 0; i < n; i += b {
			end := i + b
			if end > n {
				end = n
			}
			var ms []asymptoteobserve.LearningMemoryV1
			for j := i; j < end; j++ {
				ms = append(ms, p1Mem(j))
			}
			if err := s.PutMemories(ms); err != nil {
				t.Fatal(err)
			}
		}
		d := time.Since(start)
		results = append(results, res{fmt.Sprintf("batch-%d", b), b, float64(d.Milliseconds()), float64(d.Microseconds()) / n})
		supClose(s)
	}

	f, err := os.Create(out)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	w := csv.NewWriter(f)
	_ = w.Write([]string{"mode", "batch", "total_ms", "per_record_us", "records"})
	for _, r := range results {
		_ = w.Write([]string{r.mode, strconv.Itoa(r.batch),
			fmt.Sprintf("%.1f", r.totalMs), fmt.Sprintf("%.2f", r.perRecordU), strconv.Itoa(n)})
	}
	w.Flush()
	for _, r := range results {
		t.Logf("P1_INGEST mode=%s batch=%d total_ms=%.1f per_record_us=%.2f", r.mode, r.batch, r.totalMs, r.perRecordU)
	}
}
