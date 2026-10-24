package learning

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// Benchmarks compare the original scan path against the adaptive path on the
// same datasets. `scan` benchmarks call listMemoriesScan directly, which is the
// untouched reference. `adaptive` benchmarks call Store.ListMemories, the real
// entry point.
//
// Sizes default to 1K/10K/100K. Set BEACON_BENCH_SIZES="1000,10000,100000,1000000"
// to change them; the 1M case is expensive to build (a trigram index over the
// whole corpus) and is meant for a dedicated run.

func benchSizes() []int {
	raw := os.Getenv("BEACON_BENCH_SIZES")
	if raw == "" {
		return []int{1000, 10000, 100000}
	}
	var sizes []int
	for _, part := range splitComma(raw) {
		if n, err := strconv.Atoi(part); err == nil && n > 0 {
			sizes = append(sizes, n)
		}
	}
	if len(sizes) == 0 {
		return []int{1000, 10000, 100000}
	}
	return sizes
}

func splitComma(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	out = append(out, s[start:])
	return out
}

func benchStore(b *testing.B, n int) (*Store, *sql.DB) {
	b.Helper()
	store := Open(filepath.Join(b.TempDir(), "memory.db"))
	seedMemories(b, store, diverseDataset(n))
	db, err := store.db()
	if err != nil {
		b.Fatal(err)
	}
	if !store.ensureIndexReady(db) {
		b.Fatal("index did not become ready")
	}
	return store, db
}

func benchScan(b *testing.B, n int, query Query) {
	store, db := benchStore(b, n)
	defer db.Close()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := store.listMemoriesScan(db, query); err != nil {
			b.Fatal(err)
		}
	}
}

func benchAdaptiveCold(b *testing.B, n int, query Query) {
	store, db := benchStore(b, n)
	defer db.Close()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resetSearchCacheForTest()
		if _, err := store.ListMemories(query); err != nil {
			b.Fatal(err)
		}
	}
}

func benchAdaptiveWarm(b *testing.B, n int, query Query) {
	store, db := benchStore(b, n)
	defer db.Close()
	if _, err := store.ListMemories(query); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := store.ListMemories(query); err != nil {
			b.Fatal(err)
		}
	}
}

func selectiveQuery() Query {
	return Query{ProjectID: "proj-a", Q: "unique-needle", Limit: 5}
}

func commonQuery() Query {
	return Query{ProjectID: "proj-a", Q: "the", Limit: 5}
}

// oldOnlyQuery matches a single memory that is the oldest in the corpus, so the
// reference scan cannot terminate early: it must walk every newer row first.
func oldOnlyQuery() Query {
	return Query{ProjectID: "proj-a", Q: "zzzoldneedle", Limit: 5}
}

// noMatchQuery matches nothing, so the reference scan must read and decode every
// row in the project to prove the empty result.
func noMatchQuery() Query {
	return Query{ProjectID: "proj-a", Q: "zzznotfound", Limit: 5}
}

func BenchmarkScanSelective(b *testing.B) {
	for _, n := range benchSizes() {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) { benchScan(b, n, selectiveQuery()) })
	}
}

func BenchmarkScanCommon(b *testing.B) {
	for _, n := range benchSizes() {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) { benchScan(b, n, commonQuery()) })
	}
}

func BenchmarkAdaptiveSelectiveCold(b *testing.B) {
	for _, n := range benchSizes() {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) { benchAdaptiveCold(b, n, selectiveQuery()) })
	}
}

func BenchmarkAdaptiveSelectiveWarm(b *testing.B) {
	for _, n := range benchSizes() {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) { benchAdaptiveWarm(b, n, selectiveQuery()) })
	}
}

func BenchmarkAdaptiveCommonCold(b *testing.B) {
	for _, n := range benchSizes() {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) { benchAdaptiveCold(b, n, commonQuery()) })
	}
}

func BenchmarkScanOldOnly(b *testing.B) {
	for _, n := range benchSizes() {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) { benchScan(b, n, oldOnlyQuery()) })
	}
}

func BenchmarkAdaptiveOldOnlyCold(b *testing.B) {
	for _, n := range benchSizes() {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) { benchAdaptiveCold(b, n, oldOnlyQuery()) })
	}
}

func BenchmarkAdaptiveOldOnlyWarm(b *testing.B) {
	for _, n := range benchSizes() {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) { benchAdaptiveWarm(b, n, oldOnlyQuery()) })
	}
}

func BenchmarkScanNoMatch(b *testing.B) {
	for _, n := range benchSizes() {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) { benchScan(b, n, noMatchQuery()) })
	}
}

func BenchmarkAdaptiveNoMatchCold(b *testing.B) {
	for _, n := range benchSizes() {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) { benchAdaptiveCold(b, n, noMatchQuery()) })
	}
}

// BenchmarkIndexBuild measures the one-time cost of building the derived index
// from the authoritative table.
func BenchmarkIndexBuild(b *testing.B) {
	for _, n := range benchSizes() {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			store := Open(filepath.Join(b.TempDir(), "memory.db"))
			seedMemories(b, store, diverseDataset(n))
			db, err := store.db()
			if err != nil {
				b.Fatal(err)
			}
			defer db.Close()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := db.Exec(`UPDATE memory_index_meta SET value='0' WHERE key='index_ready'`); err != nil {
					b.Fatal(err)
				}
				if !store.ensureIndexReady(db) {
					b.Fatal("rebuild failed")
				}
			}
		})
	}
}
