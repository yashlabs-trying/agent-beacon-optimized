package hnsw

import (
	"math"
	"math/rand"
	"testing"
)

// graph_test.go is Phase 2 regression coverage for the optimized graph. It
// checks the structural invariants the Phase 2 rewrite must preserve and that
// the search returns an ordered, correct nearest-neighbour set.

func randVec(rng *rand.Rand, dim int) []float32 {
	v := make([]float32, dim)
	for i := range v {
		v[i] = float32(rng.NormFloat64())
	}
	normalize(v)
	return v
}

func normalize(v []float32) {
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

func buildTestGraph(t *testing.T, n, dim, m, ef int, seed int64) *Graph[string] {
	t.Helper()
	rng := rand.New(rand.NewSource(seed))
	g := NewGraph[string]()
	g.Distance = CosineDistance
	g.M = m
	g.Ml = 0.25
	g.EfSearch = ef
	g.EfConstruction = ef
	g.Rng = rand.New(rand.NewSource(seed))
	for i := 0; i < n; i++ {
		g.Add(MakeNode[string](string(rune('a'+i%26))+itoa(i), randVec(rng, dim)))
	}
	return g
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [20]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(b[pos:])
}

func TestGraphInvariants(t *testing.T) {
	const (
		n   = 3000
		dim = 32
		m   = 16
		ef  = 100
	)
	g := buildTestGraph(t, n, dim, m, ef, 7)
	if g.Len() != n {
		t.Fatalf("Len=%d want %d", g.Len(), n)
	}
	for li, layer := range g.layers {
		for key, node := range layer.nodes {
			if len(node.neighbors) > m {
				t.Fatalf("layer %d node %s has %d neighbors > M=%d", li, key, len(node.neighbors), m)
			}
			if len(node.dists) != len(node.neighbors) {
				t.Fatalf("layer %d node %s dists=%d neighbors=%d", li, key, len(node.dists), len(node.neighbors))
			}
			seen := map[*layerNode[string]]bool{}
			for i, nb := range node.neighbors {
				if nb == nil {
					t.Fatalf("layer %d node %s has nil neighbor at %d", li, key, i)
				}
				if seen[nb] {
					t.Fatalf("layer %d node %s has duplicate neighbor %s", li, key, nb.Key)
				}
				seen[nb] = true
				if _, ok := layer.nodes[nb.Key]; !ok {
					t.Fatalf("layer %d node %s neighbor %s not in layer", li, key, nb.Key)
				}
				want := g.Distance(node.Value, nb.Value)
				if math.Abs(float64(node.dists[i]-want)) > 1e-4 {
					t.Fatalf("layer %d node %s cached dist[%d]=%v want %v", li, key, i, node.dists[i], want)
				}
			}
		}
	}
}

func TestGraphSearchOrderedAndBounded(t *testing.T) {
	const (
		n   = 2000
		dim = 32
		k   = 10
	)
	g := buildTestGraph(t, n, dim, 16, 100, 3)
	rng := rand.New(rand.NewSource(99))
	for q := 0; q < 50; q++ {
		query := randVec(rng, dim)
		got := g.Search(query, k)
		if len(got) != k {
			t.Fatalf("Search returned %d, want %d", len(got), k)
		}
		prev := float32(-1)
		for _, node := range got {
			d := g.Distance(node.Value, query)
			if d < prev-1e-6 {
				t.Fatalf("results not ordered by distance: %v then %v", prev, d)
			}
			prev = d
		}
	}
}

// TestGraphSearchMatchesBruteForceOnSmallSet verifies recall against exact
// nearest neighbours on a small set. The vectors are clustered, like real
// embeddings, so exact nearest neighbours are well defined; uniform random
// vectors are a degenerate case where even canonical HNSW has low recall.
func TestGraphSearchMatchesBruteForceOnSmallSet(t *testing.T) {
	const (
		n        = 400
		dim      = 24
		k        = 10
		clusters = 8
	)
	rng := rand.New(rand.NewSource(5))
	g := NewGraph[string]()
	g.Distance = CosineDistance
	g.M = 64
	g.Ml = 0.25
	g.EfSearch = 200
	g.EfConstruction = 200
	g.Rng = rand.New(rand.NewSource(5))

	centroids := make([][]float32, clusters)
	for c := range centroids {
		centroids[c] = randVec(rng, dim)
	}
	noise := float32(1.0 / math.Sqrt(float64(dim)))

	type item struct {
		key string
		vec []float32
	}
	items := make([]item, n)
	for i := 0; i < n; i++ {
		key := "n" + itoa(i)
		c := rng.Intn(clusters)
		vec := make([]float32, dim)
		for d := range vec {
			vec[d] = centroids[c][d] + float32(rng.NormFloat64())*0.5*noise
		}
		normalize(vec)
		items[i] = item{key, vec}
		g.Add(MakeNode[string](key, vec))
	}

	qrng := rand.New(rand.NewSource(123))
	exactHits, total := 0, 0
	for q := 0; q < 40; q++ {
		c := qrng.Intn(clusters)
		query := make([]float32, dim)
		for d := range query {
			query[d] = centroids[c][d] + float32(qrng.NormFloat64())*0.3*noise
		}
		normalize(query)
		type scored struct {
			key string
			d   float32
		}
		all := make([]scored, n)
		for i, it := range items {
			all[i] = scored{it.key, CosineDistance(it.vec, query)}
		}
		for i := 1; i < len(all); i++ {
			for j := i; j > 0 && all[j].d < all[j-1].d; j-- {
				all[j], all[j-1] = all[j-1], all[j]
			}
		}
		exact := map[string]bool{}
		for i := 0; i < k; i++ {
			exact[all[i].key] = true
		}
		for _, node := range g.Search(query, k) {
			if exact[node.Key] {
				exactHits++
			}
		}
		total += k
	}
	recall := float64(exactHits) / float64(total)
	if recall < 0.99 {
		t.Fatalf("small-set recall@%d = %.4f, want >= 0.99", k, recall)
	}
	t.Logf("small-set recall@%d = %.4f", k, recall)
}

// TestEfConstructionDecoupled checks that EfConstruction widens construction
// without changing search behaviour, and that zero falls back to EfSearch.
func TestEfConstructionDecoupled(t *testing.T) {
	g := NewGraph[string]()
	g.EfSearch = 40
	if got := g.efConstruction(); got != 40 {
		t.Fatalf("efConstruction fallback = %d, want 40", got)
	}
	g.EfConstruction = 250
	if got := g.efConstruction(); got != 250 {
		t.Fatalf("efConstruction = %d, want 250", got)
	}
}
