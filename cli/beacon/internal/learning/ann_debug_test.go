package learning

import (
	"testing"

	"github.com/coder/hnsw"
)

func TestHNSWDebug(t *testing.T) {
	ids, vecs, queries := syntheticVectors(20000, 128, 32, 1)
	g := hnsw.NewGraph[string]()
	g.Distance = hnsw.CosineDistance
	g.M = 16
	g.Ml = 0.25
	g.EfSearch = 40
	for i := range vecs {
		g.Add(hnsw.MakeNode(ids[i], vecs[i]))
	}
	t.Logf("graph len=%d dims=%d layers=%d", g.Len(), g.Dims(), len(queries))
	for _, ef := range []int{20, 40, 100, 200, 400} {
		g.EfSearch = ef
		hits, total := 0, 0
		for _, q := range queries {
			exact := map[string]bool{}
			for _, id := range exactTopK(ids, vecs, q, 10) {
				exact[id] = true
			}
			for _, n := range g.Search(q, 10) {
				if exact[n.Key] {
					hits++
				}
			}
			total += 10
		}
		t.Logf("ef=%d recall@10=%.4f", ef, float64(hits)/float64(total))
	}
	// Sanity: cosine of the best exact neighbour for query 0.
	q := queries[0]
	ex := exactTopK(ids, vecs, q, 1)
	byID := map[string][]float32{}
	for i, id := range ids {
		byID[id] = vecs[i]
	}
	t.Logf("best exact cosine=%.4f", cosine(q, byID[ex[0]]))
}
