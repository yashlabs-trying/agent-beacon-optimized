package learning

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"runtime"
	"sort"
	"testing"
	"time"
)

func TestANNPhase1Matrix(t *testing.T) {
	if os.Getenv("BEACON_ANN_MATRIX") != "1" {
		t.Skip("set BEACON_ANN_MATRIX=1 for the 12-cell Phase 1 sweep")
	}
	const seed = int64(7)
	for _, n := range []int{100, 1000, 5000, 10000} {
		ids, vecs, queries := syntheticVectors(n, 128, 256, seed)
		queries = queries[:64]
		truth := make([][]string, len(queries))
		for i, q := range queries {
			truth[i] = exactTopK(ids, vecs, q, 10)
		}
		for _, m := range []int{16, 32, 64} {
			params := defaultANNParams()
			params.M = m
			runtime.GC()
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			started := time.Now()
			idx := newANNIndex(params)
			idx.graph.Rng = rand.New(rand.NewSource(99))
			for i, vec := range vecs {
				if !idx.add(ids[i], vec) {
					t.Fatalf("valid vector %d rejected", i)
				}
			}
			build := time.Since(started)
			runtime.ReadMemStats(&after)
			times := make([]time.Duration, len(queries))
			hit1, hit10 := 0, 0
			for i, q := range queries {
				begin := time.Now()
				got := idx.search(q, 10)
				times[i] = time.Since(begin)
				if len(got) > 0 && got[0] == truth[i][0] {
					hit1++
				}
				gold := make(map[string]bool, len(truth[i]))
				for _, id := range truth[i] {
					gold[id] = true
				}
				for _, id := range got {
					if gold[id] {
						hit10++
					}
				}
			}
			sort.Slice(times, func(i, j int) bool { return times[i] < times[j] })
			allocs := testing.AllocsPerRun(30, func() { _ = idx.search(queries[0], 10) })
			fmt.Printf("ANN_MATRIX n=%d M=%d seed=%d recall@1=%.4f recall@10=%.4f build_ms=%d p50_us=%d p95_us=%d p99_us=%d heap_mb=%.1f build_alloc_mb=%.1f search_allocs=%.1f\n",
				n, m, seed, float64(hit1)/float64(len(queries)), float64(hit10)/float64(10*len(queries)),
				build.Milliseconds(), annPercentile(times, 0.50).Microseconds(), annPercentile(times, 0.95).Microseconds(), annPercentile(times, 0.99).Microseconds(),
				float64(after.HeapAlloc)/(1024*1024), float64(after.TotalAlloc-before.TotalAlloc)/(1024*1024), allocs)
		}
	}
}

func annPercentile(values []time.Duration, p float64) time.Duration {
	index := int(math.Ceil(float64(len(values))*p)) - 1
	if index < 0 {
		index = 0
	}
	return values[index]
}

func TestANNInsertionOrderAndDuplicates(t *testing.T) {
	ids, vecs, queries := syntheticVectors(1000, 128, 32, 11)
	q := queries[0]
	for _, reverse := range []bool{false, true} {
		idx := newANNIndex(defaultANNParams())
		idx.graph.Rng = rand.New(rand.NewSource(99))
		for i := range vecs {
			at := i
			if reverse {
				at = len(vecs) - 1 - i
			}
			idx.add(ids[at], vecs[at])
		}
		idx.add("duplicate", vecs[0])
		if idx.len() != 1001 || len(idx.search(q, 10)) != 10 {
			t.Fatalf("reverse=%v: duplicate insertion or search failed", reverse)
		}
	}
}
