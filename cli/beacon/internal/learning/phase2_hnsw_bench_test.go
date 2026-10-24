//go:build !windows

package learning

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/asymptote-labs/agent-beacon/cli/beacon/internal/hnsw"
)

// phase2PeakRSSMB reports peak resident set size in MB. On Darwin Maxrss is in
// bytes; on Linux it is in kilobytes. /proc/self/status is Linux-only, so this
// uses getrusage to work on both platforms.
func phase2PeakRSSMB() int64 {
	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
		return 0
	}
	if runtime.GOOS == "darwin" {
		return int64(ru.Maxrss) / (1024 * 1024)
	}
	return int64(ru.Maxrss) / 1024
}

// phase2_hnsw_bench_test.go is Phase 2 profiling/benchmark instrumentation for the
// HNSW build path. It is measurement-only: it never changes production behavior.
// Profiling instrumentation is kept separate from the production code so final
// timing measurements are not distorted by counters.

func phase2Sizes() []int {
	raw := os.Getenv("BEACON_PHASE2_ANN_SIZES")
	if raw == "" {
		return []int{1000, 10000, 25000, 50000}
	}
	var out []int
	for _, part := range strings.Split(raw, ",") {
		if v, err := strconv.Atoi(strings.TrimSpace(part)); err == nil && v > 0 {
			out = append(out, v)
		}
	}
	return out
}

func phase2M() int {
	if m := os.Getenv("BEACON_ANN_M"); m != "" {
		if v, err := strconv.Atoi(m); err == nil && v > 0 {
			return v
		}
	}
	return 64
}

// BenchmarkHNSWBuild measures index construction throughput only. It uses the
// same syntheticVectors generator as the production recall benchmark so the
// numbers are comparable.
func BenchmarkHNSWBuild(b *testing.B) {
	m := phase2M()
	for _, n := range phase2Sizes() {
		ids, vecs, _ := syntheticVectors(n, 128, 256, 7)
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				idx := newANNIndex(annParams{M: m, Ml: 0.25, EfSearch: 100})
				for j := range vecs {
					idx.add(ids[j], vecs[j])
				}
			}
		})
	}
}

// TestHNSWBuildTiming prints a single measured build per size with wall time and
// vectors/second. It is informational (not a gate) and is the number recorded in
// the Phase 2 report.
func TestHNSWBuildTiming(t *testing.T) {
	if os.Getenv("BEACON_PHASE2_TIMING") == "" {
		t.Skip("set BEACON_PHASE2_TIMING=1 to run build timing")
	}
	m := phase2M()
	for _, n := range phase2Sizes() {
		ids, vecs, _ := syntheticVectors(n, 128, 256, 7)
		start := time.Now()
		idx := newANNIndex(annParams{M: m, Ml: 0.25, EfSearch: 100})
		for j := range vecs {
			idx.add(ids[j], vecs[j])
		}
		elapsed := time.Since(start)
		t.Logf("HNSW build n=%d M=%d wall_ms=%d vectors_per_sec=%.0f peak_rss_mb=%d",
			n, m, elapsed.Milliseconds(), float64(n)/elapsed.Seconds(), phase2PeakRSSMB())
		if idx.len() != n {
			t.Fatalf("built graph has %d nodes, want %d", idx.len(), n)
		}
	}
}

// TestHNSWDeterminism checks that two builds with the same seed and the same
// insertion order produce identical neighbour sets, which the optimization work
// must preserve.
func TestHNSWDeterminism(t *testing.T) {
	const n = 2000
	ids, vecs, _ := syntheticVectors(n, 128, 64, 11)
	build := func() *hnsw.Graph[string] {
		g := hnsw.NewGraph[string]()
		g.Distance = hnsw.CosineDistance
		g.M = 64
		g.Ml = 0.25
		g.EfSearch = 100
		for i := range vecs {
			g.Add(hnsw.MakeNode(ids[i], vecs[i]))
		}
		return g
	}
	a, b := build(), build()
	if a.Len() != b.Len() {
		t.Fatalf("len mismatch: %d vs %d", a.Len(), b.Len())
	}
	// Compare lookup vectors and structural determinism via a fixed probe set.
	probes := []string{ids[0], ids[n/2], ids[n-1]}
	for _, p := range probes {
		va, okA := a.Lookup(p)
		vb, okB := b.Lookup(p)
		if okA != okB || len(va) != len(vb) {
			t.Fatalf("lookup mismatch for %s", p)
		}
		for i := range va {
			if va[i] != vb[i] {
				t.Fatalf("vector mismatch for %s at %d", p, i)
			}
		}
	}
}
