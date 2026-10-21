package learning

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/asymptote-labs/agent-beacon/pkg/asymptoteobserve"
)

// This file is a measurement harness. It is skipped unless BEACON_VERIFY is set,
// so it never runs in an ordinary `go test`. It is self-contained: the only
// tree-specific symbols it uses are verifyResetCache and verifyPrepareIndex,
// which live in search_verify_hooks_test.go. The same harness source is dropped
// into the untouched baseline (with baseline hooks) so both trees are measured
// by identical code, datasets and query distributions.
//
// Output lines are stable and grep-friendly:
//   VERIFY_SIZE tree=.. n=.. db_bytes=.. seed_ms=.. build_ms=..
//   VERIFY tree=.. n=.. workload=.. mode=.. iters=.. p50_us=.. p95_us=.. p99_us=.. mean_us=.. alloc_kb=..

func verifyEnabled(tb testing.TB) bool {
	if os.Getenv("BEACON_VERIFY") == "" {
		tb.Skip("set BEACON_VERIFY=1 to run the verification harness")
		return false
	}
	return true
}

func verifyTree() string {
	if v := os.Getenv("BEACON_VERIFY_TREE"); v != "" {
		return v
	}
	return "unknown"
}

func verifySizes() []int {
	raw := os.Getenv("BEACON_VERIFY_SIZES")
	if raw == "" {
		return []int{1000, 10000, 100000, 1000000}
	}
	var out []int
	for _, part := range strings.Split(raw, ",") {
		if n, err := strconv.Atoi(strings.TrimSpace(part)); err == nil && n > 0 {
			out = append(out, n)
		}
	}
	if len(out) == 0 {
		return []int{1000, 10000, 100000, 1000000}
	}
	return out
}

func verifyIters(n int, cold bool) int {
	if override := verifyIterOverride(); override > 0 {
		return override
	}
	switch {
	case n <= 10000:
		if cold {
			return 200
		}
		return 1000
	case n <= 100000:
		if cold {
			return 30
		}
		return 200
	default:
		if cold {
			return 3
		}
		return 10
	}
}

func verifyIterOverride() int {
	if n, err := strconv.Atoi(os.Getenv("BEACON_VERIFY_ITERS")); err == nil && n > 0 {
		return n
	}
	return 0
}

func verifyOnly() map[string]bool {
	raw := os.Getenv("BEACON_VERIFY_ONLY")
	if raw == "" {
		return nil
	}
	out := map[string]bool{}
	for _, part := range strings.Split(raw, ",") {
		out[strings.TrimSpace(part)] = true
	}
	return out
}

type verifyWorkload struct {
	name  string
	query Query
}

func verifyWorkloads() []verifyWorkload {
	return []verifyWorkload{
		{"selective", Query{ProjectID: "proj-a", Q: "unique-needle", Limit: 5}},
		{"old-only", Query{ProjectID: "proj-a", Q: "zzzoldneedle", Limit: 5}},
		{"no-match", Query{ProjectID: "proj-a", Q: "zzznotfound", Limit: 5}},
		{"common", Query{ProjectID: "proj-a", Q: "the", Limit: 5}},
		{"empty", Query{ProjectID: "proj-a", Limit: 5}},
		{"mixed", Query{ProjectID: "proj-a", Q: "workflow", Kind: asymptoteobserve.LearningMemoryKindConvention, Limit: 5}},
	}
}

func verifyTimestamp(i int) string {
	return time.Unix(1700000000+int64(i), 0).UTC().Format(time.RFC3339Nano)
}

func verifyDataset(n int) []asymptoteobserve.LearningMemoryV1 {
	projects := []string{"proj-a", "proj-a", "proj-a", "proj-b"}
	kinds := []string{
		asymptoteobserve.LearningMemoryKindConvention,
		asymptoteobserve.LearningMemoryKindDebuggingPattern,
	}
	out := make([]asymptoteobserve.LearningMemoryV1, 0, n)
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("memory-%07d", i)
		title := fmt.Sprintf("Note %d about the workflow", i)
		body := "The agent should follow the ordinary the steps for the task."
		switch {
		case i == 1:
			body = "The zzzoldneedle recovery note for a long-past incident."
		case i%500 == 0:
			body = "Remember the rare unique-needle recovery step for the outage."
		case i%37 == 0:
			body = "Run the package smoke test again after a transient fetch failure."
		case i%53 == 0:
			body = "Prefer café-style Unicode labels here."
		case i%71 == 0:
			body = "Use the foo-bar flag together with foo.baz."
		}
		project := projects[i%len(projects)]
		m := asymptoteobserve.LearningMemoryV1{
			ID:          id,
			CandidateID: "candidate-" + id,
			Kind:        kinds[i%len(kinds)],
			Title:       title,
			Body:        body,
			Project:     asymptoteobserve.LearningProjectV1{ID: project, Path: "/repos/" + project},
			CreatedAt:   verifyTimestamp(i),
			UpdatedAt:   verifyTimestamp(i),
		}
		if i%97 == 0 && i > 0 {
			m.SupersededBy = fmt.Sprintf("memory-%07d", i-1)
		}
		out = append(out, m)
	}
	return out
}

func verifySeed(tb testing.TB, store *Store, memories []asymptoteobserve.LearningMemoryV1) {
	tb.Helper()
	db, err := store.db()
	if err != nil {
		tb.Fatal(err)
	}
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		tb.Fatal(err)
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`INSERT INTO memories (id, candidate_id, project_id, kind, title, created_at, updated_at, superseded_by, memory_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		tb.Fatal(err)
	}
	defer stmt.Close()
	for _, m := range memories {
		data, err := json.Marshal(m)
		if err != nil {
			tb.Fatal(err)
		}
		if _, err := stmt.Exec(m.ID, m.CandidateID, m.Project.ID, m.Kind, m.Title, m.CreatedAt, m.UpdatedAt, m.SupersededBy, string(data)); err != nil {
			tb.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		tb.Fatal(err)
	}
}

func verifyPercentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(math.Ceil(p/100*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func verifyMeasure(tb testing.TB, store *Store, q Query, iters int, cold bool) (p50, p95, p99, mean, allocKB float64) {
	tb.Helper()
	if _, err := store.ListMemories(q); err != nil {
		tb.Fatal(err)
	}
	durs := make([]float64, 0, iters)
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	for i := 0; i < iters; i++ {
		if cold {
			verifyResetCache()
		}
		start := time.Now()
		if _, err := store.ListMemories(q); err != nil {
			tb.Fatal(err)
		}
		durs = append(durs, float64(time.Since(start).Microseconds()))
	}
	runtime.ReadMemStats(&after)
	sort.Float64s(durs)
	var sum float64
	for _, d := range durs {
		sum += d
	}
	return verifyPercentile(durs, 50), verifyPercentile(durs, 95), verifyPercentile(durs, 99),
		sum / float64(len(durs)),
		float64(after.TotalAlloc-before.TotalAlloc) / float64(iters) / 1024
}

func TestVerifyHarness(t *testing.T) {
	if !verifyEnabled(t) {
		return
	}
	tree := verifyTree()
	for _, n := range verifySizes() {
		store := Open(filepath.Join(t.TempDir(), "memory.db"))
		seedStart := time.Now()
		verifySeed(t, store, verifyDataset(n))
		seedMS := time.Since(seedStart).Milliseconds()

		db, err := store.db()
		if err != nil {
			t.Fatal(err)
		}
		buildStart := time.Now()
		verifyPrepareIndex(t, store, db)
		buildMS := time.Since(buildStart).Milliseconds()

		var pageCount, pageSize int64
		if err := db.QueryRow(`PRAGMA page_count`).Scan(&pageCount); err != nil {
			t.Fatal(err)
		}
		if err := db.QueryRow(`PRAGMA page_size`).Scan(&pageSize); err != nil {
			t.Fatal(err)
		}
		fmt.Printf("VERIFY_SIZE tree=%s n=%d db_bytes=%d seed_ms=%d build_ms=%d\n",
			tree, n, pageCount*pageSize, seedMS, buildMS)

		for _, wl := range verifyWorkloads() {
			if only := verifyOnly(); only != nil && !only[wl.name] {
				continue
			}
			for _, mode := range []struct {
				name string
				cold bool
			}{{"cold", true}, {"warm", false}} {
				iters := verifyIters(n, mode.cold)
				p50, p95, p99, mean, allocKB := verifyMeasure(t, store, wl.query, iters, mode.cold)
				fmt.Printf("VERIFY tree=%s n=%d workload=%s mode=%s iters=%d p50_us=%.1f p95_us=%.1f p99_us=%.1f mean_us=%.1f alloc_kb=%.2f\n",
					tree, n, wl.name, mode.name, iters, p50, p95, p99, mean, allocKB)
			}
		}
		db.Close()
	}
}

// TestVerifyWAL measures database and WAL file growth from incremental writes
// and the effect of a truncating checkpoint.
func TestVerifyWAL(t *testing.T) {
	if !verifyEnabled(t) {
		return
	}
	tree := verifyTree()
	dir := t.TempDir()
	path := filepath.Join(dir, "memory.db")
	store := Open(path)
	project := asymptoteobserve.LearningProjectV1{ID: "proj-a"}
	const writes = 5000
	writeStart := time.Now()
	for i := 0; i < writes; i++ {
		if err := store.PutMemory(asymptoteobserve.LearningMemoryV1{
			ID: fmt.Sprintf("w-%06d", i), CandidateID: fmt.Sprintf("wc-%06d", i),
			Kind:  asymptoteobserve.LearningMemoryKindConvention,
			Title: fmt.Sprintf("Write %d", i), Body: "the needle incremental write body",
			Project: project,
		}); err != nil {
			t.Fatal(err)
		}
	}
	writeMS := time.Since(writeStart).Milliseconds()
	size := func(suffix string) int64 {
		info, err := os.Stat(path + suffix)
		if err != nil {
			return 0
		}
		return info.Size()
	}
	fmt.Printf("VERIFY_WAL tree=%s writes=%d write_ms=%d per_write_us=%.1f db_bytes=%d wal_bytes=%d shm_bytes=%d\n",
		tree, writes, writeMS, float64(writeMS)*1000/float64(writes), size(""), size("-wal"), size("-shm"))

	db, err := store.db()
	if err != nil {
		t.Fatal(err)
	}
	var busy, logFrames, checkpointed int
	if err := db.QueryRow(`PRAGMA wal_checkpoint(TRUNCATE)`).Scan(&busy, &logFrames, &checkpointed); err != nil {
		t.Fatal(err)
	}
	db.Close()
	fmt.Printf("VERIFY_WAL tree=%s checkpoint=truncate busy=%d log=%d checkpointed=%d db_bytes=%d wal_bytes=%d\n",
		tree, busy, logFrames, checkpointed, size(""), size("-wal"))
}
