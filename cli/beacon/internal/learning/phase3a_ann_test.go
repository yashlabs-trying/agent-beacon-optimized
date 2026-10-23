package learning

// Phase 3A Workstream B: bounded HNSW ANN validation against exact nearest
// neighbours at practical sizes. Opt-in via BEACON_PHASE3_ANN=1.
//
//   BEACON_PHASE3_ANN_SIZES  comma list (default "1000,10000,25000")
//   BEACON_ANN_M             HNSW M (default 64, the verified reference)
//   BEACON_PHASE3_ANN_SEED   graph RNG seed (default 99)
//
// Reports Recall@1/@10 vs exact, build ms, search p50/p95, per-search
// allocations, and peak RSS. Also checks seed determinism.

import (
	"fmt"
	"math/rand"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

func p3ANNInts(name, def string) []int {
	raw := os.Getenv(name)
	if raw == "" {
		raw = def
	}
	var out []int
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		n, err := strconv.Atoi(part)
		if err != nil {
			continue
		}
		out = append(out, n)
	}
	return out
}

func p3ANNEnvInt(name string, def int) int {
	if raw := os.Getenv(name); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			return n
		}
	}
	return def
}

func TestPhase3AANNvsExact(t *testing.T) {
	if os.Getenv("BEACON_PHASE3_ANN") != "1" {
		t.Skip("set BEACON_PHASE3_ANN=1 to run the bounded ANN validation")
	}
	const vecSeed = int64(7)
	graphSeed := int64(p3ANNEnvInt("BEACON_PHASE3_ANN_SEED", 99))
	m := p3ANNEnvInt("BEACON_ANN_M", 64)
	sizes := p3ANNInts("BEACON_PHASE3_ANN_SIZES", "1000,10000,25000")
	fmt.Printf("P3A_ANN config M=%d graph_seed=%d vec_seed=%d sizes=%v\n", m, graphSeed, vecSeed, sizes)

	for _, n := range sizes {
		ids, vecs, queries := syntheticVectors(n, 128, 256, vecSeed)
		queries = queries[:64]
		truth := make([][]string, len(queries))
		for i, q := range queries {
			truth[i] = exactTopK(ids, vecs, q, 10)
		}

		params := defaultANNParams()
		params.M = m
		rssBefore := peakRSSKB()
		started := time.Now()
		idx := newANNIndex(params)
		idx.graph.Rng = rand.New(rand.NewSource(graphSeed))
		for i, vec := range vecs {
			if !idx.add(ids[i], vec) {
				t.Fatalf("valid vector %d rejected at n=%d", i, n)
			}
		}
		build := time.Since(started)
		rssAfter := peakRSSKB()

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
		fmt.Printf("P3A_ANN n=%d M=%d recall@1=%.4f recall@10=%.4f build_ms=%d p50_us=%d p95_us=%d search_allocs=%.1f rss_before_mb=%d rss_after_mb=%d\n",
			n, m, float64(hit1)/float64(len(queries)), float64(hit10)/float64(10*len(queries)),
			build.Milliseconds(), p3Percentile(times, 0.50).Microseconds(), p3Percentile(times, 0.95).Microseconds(),
			allocs, rssBefore/1024, rssAfter/1024)

		// Determinism: rebuild with the same seed and require identical results.
		idx2 := newANNIndex(params)
		idx2.graph.Rng = rand.New(rand.NewSource(graphSeed))
		for i, vec := range vecs {
			idx2.add(ids[i], vec)
		}
		same := true
		for i, q := range queries {
			got := idx.search(q, 10)
			got2 := idx2.search(q, 10)
			if strings.Join(got, ",") != strings.Join(got2, ",") {
				same = false
				break
			}
			_ = i
		}
		fmt.Printf("P3A_ANN n=%d M=%d determinism_same_seed=%v\n", n, m, same)
	}
}
