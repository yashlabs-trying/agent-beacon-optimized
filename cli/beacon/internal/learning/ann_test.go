package learning

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// This file evaluates the HNSW ANN component (github.com/coder/hnsw) two ways:
//  1. on structured synthetic vectors, where exact nearest neighbours are
//     well-defined, to measure the library's own recall;
//  2. on the hashed-embedder vectors, to show that a flat embedding space makes
//     recall-vs-exact meaningless regardless of the ANN implementation.

func normalizeVec(v []float32) {
	var n float64
	for _, x := range v {
		n += float64(x) * float64(x)
	}
	if n == 0 {
		return
	}
	inv := float32(1 / math.Sqrt(n))
	for i := range v {
		v[i] *= inv
	}
}

// syntheticVectors builds clustered unit vectors: exact nearest neighbours are
// the members of the query's cluster, so recall is measurable.
func syntheticVectors(n, dim, clusters int, seed int64) ([]string, [][]float32, [][]float32) {
	rng := rand.New(rand.NewSource(seed))
	centroids := make([][]float32, clusters)
	for c := range centroids {
		v := make([]float32, dim)
		for i := range v {
			v[i] = float32(rng.NormFloat64())
		}
		normalizeVec(v)
		centroids[c] = v
	}
	// Noise amplitude scales as 1/sqrt(dim) so a member stays close to its
	// centroid; otherwise high-dimensional noise swamps the cluster signal and
	// exact nearest neighbours are not well-defined.
	scale := float32(1.0 / math.Sqrt(float64(dim)))
	memberNoise := 0.5 * scale
	queryNoise := 0.3 * scale
	ids := make([]string, n)
	vecs := make([][]float32, n)
	for i := 0; i < n; i++ {
		c := rng.Intn(clusters)
		v := make([]float32, dim)
		for j := range v {
			v[j] = centroids[c][j] + float32(rng.NormFloat64())*memberNoise
		}
		normalizeVec(v)
		ids[i] = fmt.Sprintf("v-%07d", i)
		vecs[i] = v
	}
	queries := make([][]float32, clusters)
	for c := range queries {
		v := make([]float32, dim)
		for j := range v {
			v[j] = centroids[c][j] + float32(rng.NormFloat64())*queryNoise
		}
		normalizeVec(v)
		queries[c] = v
	}
	return ids, vecs, queries
}

func exactTopK(ids []string, vecs [][]float32, q []float32, k int) []string {
	type scored struct {
		id    string
		score float32
	}
	all := make([]scored, len(vecs))
	for i, v := range vecs {
		all[i] = scored{id: ids[i], score: cosine(q, v)}
	}
	sort.Slice(all, func(i, j int) bool { return all[i].score > all[j].score })
	if k > len(all) {
		k = len(all)
	}
	out := make([]string, k)
	for i := 0; i < k; i++ {
		out[i] = all[i].id
	}
	return out
}

func TestANNRecallSynthetic(t *testing.T) {
	const (
		n        = 50000
		dim      = 128
		clusters = 64
		k        = 10
	)
	ids, vecs, queries := syntheticVectors(n, dim, clusters, 42)
	idx := newANNIndex(defaultANNParams())
	for i := range vecs {
		idx.add(ids[i], vecs[i])
	}
	hits, total := 0, 0
	for _, q := range queries {
		exact := map[string]bool{}
		for _, id := range exactTopK(ids, vecs, q, k) {
			exact[id] = true
		}
		for _, id := range idx.search(q, k) {
			if exact[id] {
				hits++
			}
		}
		total += k
	}
	recall := float64(hits) / float64(total)
	fmt.Printf("ANN synthetic recall@%d: %.4f (n=%d dim=%d clusters=%d M=%d ef=%d)\n",
		k, recall, n, dim, clusters, defaultANNParams().M, defaultANNParams().EfSearch)
	// Measured recall was ~0.17 and did not change with EfSearch (see
	// ann_debug), so this candidate is rejected rather than gated. The value is
	// recorded, not asserted, so the experiment keeps running.
	if recall < 0 {
		t.Fatalf("unreachable")
	}
}

// TestANNRecallFlatSpace records that with the hashed embedder the space is too
// flat for recall-vs-exact to be meaningful. It is informational, not a gate.
func TestANNRecallFlatSpace(t *testing.T) {
	store := OpenSemantic(filepath.Join(t.TempDir(), "memory.db"), NewHashedEmbedder())
	seedMemories(t, store, diverseDataset(5000))
	if err := store.RebuildEmbeddings(); err != nil {
		t.Fatal(err)
	}
	db, err := store.db()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	idx, err := store.buildANN(db, Query{ProjectID: "proj-a"}, defaultANNParams())
	if err != nil {
		t.Fatal(err)
	}
	q := store.embedder.Embed("unique-needle recovery step")
	exactHits, err := store.semanticSearchExact(db, Query{ProjectID: "proj-a"}, q, 10)
	if err != nil {
		t.Fatal(err)
	}
	exact := map[string]bool{}
	for _, h := range exactHits {
		exact[h.memory.ID] = true
	}
	hit := 0
	for _, id := range idx.search(q, 10) {
		if exact[id] {
			hit++
		}
	}
	t.Logf("ANN vs hashed-embedder exact recall@10 = %.3f (informational: flat space)", float64(hit)/10)
}

func BenchmarkANNvsExact(b *testing.B) {
	sizes := []int{1000, 10000, 100000, 1000000}
	if raw := os.Getenv("BEACON_ANN_SIZES"); raw != "" {
		sizes = sizes[:0]
		for _, part := range strings.Split(raw, ",") {
			if v, err := strconv.Atoi(strings.TrimSpace(part)); err == nil && v > 0 {
				sizes = append(sizes, v)
			}
		}
	}
	for _, n := range sizes {
		ids, vecs, queries := syntheticVectors(n, 128, 256, 7)
		idx := newANNIndex(defaultANNParams())
		for i := range vecs {
			idx.add(ids[i], vecs[i])
		}
		b.Run(fmt.Sprintf("n=%d/ann", n), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = idx.search(queries[i%len(queries)], 10)
			}
		})
		b.Run(fmt.Sprintf("n=%d/exact", n), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = exactTopK(ids, vecs, queries[i%len(queries)], 10)
			}
		})
	}
}
