package learning

import (
	"database/sql"
	"os"
	"strings"
	"sync"

	"github.com/coder/hnsw"
)

// This file is the v4 experiment for approximate nearest-neighbour (ANN)
// retrieval, using the pure-Go HNSW implementation github.com/coder/hnsw
// (v0.6.1, MIT). It is additive and experimental: nothing calls it unless an
// embedder is configured and the caller asks for the ANN path.
//
// The graph is keyed by memory ID and scoped per project, so a search can never
// return a memory from another project even though the index holds vectors.
// Indexes are rebuilt when the durable generation changes; persistence is
// available through Export/Import but is not automatically loaded here.

// annParams are the HNSW build/search parameters. M and Ml follow the library's
// guidance; EfSearch trades recall for latency.
type annParams struct {
	M        int
	Ml       float64
	EfSearch int
}

func defaultANNParams() annParams {
	return annParams{M: 16, Ml: 0.25, EfSearch: 40}
}

// annIndex wraps an HNSW graph keyed by memory ID.
type annIndex struct {
	graph *hnsw.Graph[string]
}

func newANNIndex(p annParams) *annIndex {
	g := hnsw.NewGraph[string]()
	g.Distance = hnsw.CosineDistance
	g.M = p.M
	g.Ml = p.Ml
	g.EfSearch = p.EfSearch
	return &annIndex{graph: g}
}

func (a *annIndex) add(id string, vec []float32) {
	a.graph.Add(hnsw.MakeNode(id, vec))
}

func (a *annIndex) search(qvec []float32, k int) []string {
	nodes := a.graph.Search(qvec, k)
	out := make([]string, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, n.Key)
	}
	return out
}

func (a *annIndex) len() int { return a.graph.Len() }

func (a *annIndex) export(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return a.graph.Export(f)
}

func (a *annIndex) load(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return a.graph.Import(f)
}

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
	for rows.Next() {
		var id string
		var blob []byte
		var dim int
		if err := rows.Scan(&id, &blob, &dim); err != nil {
			return nil, err
		}
		vec := decodeVector(blob)
		if vec == nil || dim != len(vec) {
			continue
		}
		idx.add(id, vec)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return idx, nil
}

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
