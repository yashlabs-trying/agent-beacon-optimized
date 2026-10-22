package learning

import (
	"database/sql"
	"math"
	"math/rand"
	"strings"
	"sync"

	"github.com/asymptote-labs/agent-beacon/cli/beacon/internal/hnsw"
)

// This file is the ANN retrieval layer. It uses a vendored, corrected copy of
// github.com/coder/hnsw v0.6.1 (MIT) in internal/hnsw: the upstream layer search
// stopped after the entry node's neighbours (recall@10 measured at 0.17), which
// the vendored copy fixes. It is additive and experimental: nothing calls it
// unless an embedder is configured and the caller asks for the ANN path.
//
// The graph is keyed by memory ID and scoped per project, so a search can never
// return a memory from another project even though the index holds vectors.
// Indexes are rebuilt when the durable generation changes.

// annParams are the HNSW build/search parameters. M and Ml follow the library's
// guidance; EfSearch trades recall for latency.
type annParams struct {
	M        int
	Ml       float64
	EfSearch int
	// EfConstruction is the build-time beam width. Zero means use EfSearch.
	EfConstruction int
	// Seed, when non-zero, makes level generation reproducible for benchmarks.
	Seed int64
}

func defaultANNParams() annParams {
	// M=64 was chosen from a measured sweep (n=20000, 128 dims, 32 clusters):
	// M=16 recall@10=0.74, M=32 0.92-0.94, M=64 0.994-0.997. M=64 meets the
	// >=0.95 target; EfSearch has little effect once M is large enough.
	return annParams{M: 64, Ml: 0.25, EfSearch: 100}
}

// annIndex wraps an HNSW graph keyed by memory ID.
type annIndex struct {
	graph *hnsw.Graph[string]
	dim   int
}

func newANNIndex(p annParams) *annIndex {
	g := hnsw.NewGraph[string]()
	g.Distance = hnsw.CosineDistance
	g.M = p.M
	g.Ml = p.Ml
	g.EfSearch = p.EfSearch
	g.EfConstruction = p.EfConstruction
	if p.Seed != 0 {
		g.Rng = rand.New(rand.NewSource(p.Seed))
	}
	return &annIndex{graph: g}
}

func validANNVector(vec []float32) bool {
	if len(vec) == 0 {
		return false
	}
	var norm float64
	for _, value := range vec {
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			return false
		}
		norm += float64(value) * float64(value)
	}
	return norm > 0 && !math.IsInf(norm, 0)
}

func (a *annIndex) add(id string, vec []float32) bool {
	if !validANNVector(vec) || (a.dim != 0 && len(vec) != a.dim) {
		return false
	}
	if a.dim == 0 {
		a.dim = len(vec)
	}
	a.graph.Add(hnsw.MakeNode(id, vec))
	return true
}

func (a *annIndex) search(qvec []float32, k int) []string {
	if k <= 0 || a.dim == 0 || len(qvec) != a.dim || !validANNVector(qvec) {
		return nil
	}
	nodes := a.graph.Search(qvec, k)
	out := make([]string, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, n.Key)
	}
	return out
}

func (a *annIndex) len() int { return a.graph.Len() }

type annCacheEntry struct {
	index      *annIndex
	generation string
}

// annCaches is keyed by dbPath, project and model, so one project's index is
// never consulted for another project's query.
var annCaches sync.Map

func (s *Store) annCacheKey(query Query) string {
	return s.dbPath + "\x00" + strings.TrimSpace(query.ProjectID) + "\x00" + s.embedder.Model()
}

// buildANN loads a project's embeddings and builds an in-memory HNSW graph.
func (s *Store) buildANN(db *sql.DB, query Query, params annParams) (*annIndex, error) {
	clauses := []string{`e.model = ?`}
	args := []any{s.embedder.Model()}
	if projectID := strings.TrimSpace(query.ProjectID); projectID != "" {
		clauses = append(clauses, `e.project_id = ?`)
		args = append(args, projectID)
	}
	if kind := strings.TrimSpace(query.Kind); kind != "" {
		clauses = append(clauses, `m.kind = ?`)
		args = append(args, kind)
	}
	clauses = append(clauses, `(m.superseded_by IS NULL OR m.superseded_by = '')`)
	stmt := `SELECT e.memory_id, e.vector, e.dim
		FROM memory_embeddings e JOIN memories m ON m.id = e.memory_id
		WHERE ` + strings.Join(clauses, ` AND `)
	rows, err := db.Query(stmt, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	idx := newANNIndex(params)
	added := 0
	for rows.Next() {
		var id string
		var blob []byte
		var dim int
		if err := rows.Scan(&id, &blob, &dim); err != nil {
			return nil, err
		}
		vec := decodeVector(blob)
		if vec == nil || dim != len(vec) || dim != s.embedder.Dim() {
			continue
		}
		if !idx.add(id, vec) {
			continue
		}
		added++
		if annBuildHook != nil && added%annBuildHookEvery == 0 {
			annBuildHook(added)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if annBuildHook != nil {
		annBuildHook(added)
	}
	return idx, nil
}

// annBuildHook, when set, is called with the number of nodes added so far during
// a graph build. It is a test-only progress hook and is nil in production.
var annBuildHook func(added int)

const annBuildHookEvery = 100000

// semanticSearchANN returns the top-k memory IDs by approximate nearest
// neighbour search, rebuilding the graph when the durable generation changes.
func (s *Store) semanticSearchANN(db *sql.DB, query Query, qvec []float32, k int, params annParams) ([]string, error) {
	if s.embedder == nil || len(qvec) != s.embedder.Dim() || k <= 0 {
		return nil, nil
	}
	key := s.annCacheKey(query)
	generation, _ := currentGeneration(db)
	if v, ok := annCaches.Load(key); ok {
		if entry, ok := v.(*annCacheEntry); ok && entry.generation == generation {
			return entry.index.search(qvec, k), nil
		}
	}
	idx, err := s.buildANN(db, query, params)
	if err != nil {
		return nil, err
	}
	annCaches.Store(key, &annCacheEntry{index: idx, generation: generation})
	return idx.search(qvec, k), nil
}

// semanticSearchANNWithIndex lets a benchmark reuse a prebuilt index without the
// process cache.
func (s *Store) semanticSearchANNWithIndex(idx *annIndex, qvec []float32, k int) []string {
	return idx.search(qvec, k)
}
