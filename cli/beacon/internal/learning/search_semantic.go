package learning

import (
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"math"
	"sort"
	"strings"

	"github.com/asymptote-labs/agent-beacon/pkg/asymptoteobserve"
)

// This file adds the optional semantic layer: embedding storage, incremental
// updates, exact vector search, and hybrid lexical+semantic fusion. It is inert
// unless a Store is opened with an Embedder (OpenSemantic). The default Open
// path is unchanged, so lexical search and the public contract are preserved.

// ensureSemanticSchema creates the embedding table. It is a derived table: rows
// can be dropped and recomputed from `memories` at any time.
func ensureSemanticSchema(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS memory_embeddings (
			memory_id TEXT PRIMARY KEY,
			project_id TEXT NOT NULL,
			model TEXT NOT NULL,
			content_hash TEXT NOT NULL,
			dim INTEGER NOT NULL,
			vector BLOB NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS memory_embeddings_by_project ON memory_embeddings (project_id, model)`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

// OpenSemantic opens a store with the semantic layer enabled using the given
// embedder. Passing nil is equivalent to Open.
func OpenSemantic(path string, embedder Embedder) *Store {
	if path == "" {
		path = storeFile
	}
	return &Store{dbPath: path, embedder: embedder}
}

func embeddingContentHash(m asymptoteobserve.LearningMemoryV1) string {
	sum := sha256.Sum256([]byte(memoryHaystack(m)))
	return hex.EncodeToString(sum[:16])
}

func encodeVector(vec []float32) []byte {
	buf := make([]byte, 4*len(vec))
	for i, v := range vec {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(v))
	}
	return buf
}

func decodeVector(buf []byte) []float32 {
	if len(buf)%4 != 0 {
		return nil
	}
	out := make([]float32, len(buf)/4)
	for i := range out {
		out[i] = math.Float32frombits(binary.LittleEndian.Uint32(buf[i*4:]))
	}
	return out
}

// upsertEmbeddingTx keeps one memory's embedding current inside the write
// transaction. It is incremental: a memory whose content and model are unchanged
// is not re-embedded. Errors are returned so the caller can decide, but the
// caller treats embedding as derived state and never fails the write for it.
func (s *Store) upsertEmbeddingTx(tx *sql.Tx, m asymptoteobserve.LearningMemoryV1) error {
	if s.embedder == nil {
		return nil
	}
	hash := embeddingContentHash(m)
	var existingHash, existingModel string
	err := tx.QueryRow(`SELECT content_hash, model FROM memory_embeddings WHERE memory_id = ?`, m.ID).Scan(&existingHash, &existingModel)
	switch {
	case err == nil:
		if existingHash == hash && existingModel == s.embedder.Model() {
			return nil
		}
	case err != sql.ErrNoRows:
		return err
	}
	vec := s.embedder.Embed(memoryHaystack(m))
	_, err = tx.Exec(`INSERT INTO memory_embeddings (memory_id, project_id, model, content_hash, dim, vector)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(memory_id) DO UPDATE SET
			project_id = excluded.project_id, model = excluded.model,
			content_hash = excluded.content_hash, dim = excluded.dim, vector = excluded.vector`,
		m.ID, m.Project.ID, s.embedder.Model(), hash, len(vec), encodeVector(vec))
	return err
}

type semanticHit struct {
	memory asymptoteobserve.LearningMemoryV1
	score  float32
}

// semanticSearchExact returns the top-k memories by cosine similarity to the
// query vector, scoped to the project (and kind) and excluding superseded rows.
// It is an exact nearest-neighbour search: it reads every candidate vector. This
// is the correctness reference and the small-store mode; approximate search
// would be added only above a measured break-even point.
func (s *Store) semanticSearchExact(db *sql.DB, query Query, qvec []float32, k int) ([]semanticHit, error) {
	if s.embedder == nil || len(qvec) != s.embedder.Dim() || k <= 0 {
		return nil, nil
	}
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
	stmt := `SELECT e.vector, e.dim, m.memory_json
		FROM memory_embeddings e
		JOIN memories m ON m.id = e.memory_id
		WHERE ` + strings.Join(clauses, ` AND `)
	rows, err := db.Query(stmt, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	best := make([]semanticHit, 0, k)
	for rows.Next() {
		var blob []byte
		var dim int
		var raw string
		if err := rows.Scan(&blob, &dim, &raw); err != nil {
			return nil, err
		}
		if dim != len(qvec) {
			continue
		}
		vec := decodeVector(blob)
		if vec == nil {
			continue
		}
		score := cosine(qvec, vec)
		if len(best) < k {
			var m asymptoteobserve.LearningMemoryV1
			if err := json.Unmarshal([]byte(raw), &m); err != nil {
				return nil, err
			}
			best = append(best, semanticHit{memory: m, score: score})
			if len(best) == k {
				sort.Slice(best, func(i, j int) bool { return best[i].score < best[j].score })
			}
			continue
		}
		if score <= best[0].score {
			continue
		}
		var m asymptoteobserve.LearningMemoryV1
		if err := json.Unmarshal([]byte(raw), &m); err != nil {
			return nil, err
		}
		best[0] = semanticHit{memory: m, score: score}
		// Sift the new minimum into place.
		for i := 1; i < len(best); i++ {
			if best[i].score < best[0].score {
				best[0], best[i] = best[i], best[0]
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Slice(best, func(i, j int) bool { return best[i].score > best[j].score })
	return best, nil
}

// memoriesByIDs hydrates memory IDs returned by the ANN index into memories,
// applying the same supersession filter as the other paths and preserving the
// ANN rank order. Missing or superseded IDs are dropped.
func (s *Store) memoriesByIDs(db *sql.DB, ids []string) ([]semanticHit, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, err := db.Query(`SELECT id, memory_json FROM memories WHERE id IN (`+placeholders+`) AND (superseded_by IS NULL OR superseded_by = '')`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byID := map[string]asymptoteobserve.LearningMemoryV1{}
	for rows.Next() {
		var id, raw string
		if err := rows.Scan(&id, &raw); err != nil {
			return nil, err
		}
		var m asymptoteobserve.LearningMemoryV1
		if err := json.Unmarshal([]byte(raw), &m); err != nil {
			return nil, err
		}
		byID[id] = m
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]semanticHit, 0, len(ids))
	for rank, id := range ids {
		if m, ok := byID[id]; ok {
			out = append(out, semanticHit{memory: m, score: float32(-rank)})
		}
	}
	return out, nil
}

// HybridOptions configures SearchHybrid.
type HybridOptions struct {
	// Limit is the number of fused results returned.
	Limit int
	// CandidateK is how many candidates each source contributes.
	CandidateK int
	// RRFK is the reciprocal-rank-fusion constant (60 is standard).
	RRFK float64
	// UseANN selects the HNSW approximate semantic source instead of exact
	// cosine search. It requires an embedder; ignored otherwise.
	UseANN bool
	// LexicalWeight and SemanticWeight scale each source's reciprocal-rank
	// contribution. Zero means the default of 1, which reproduces standard RRF.
	// They exist so a weighting can be tuned and validated; the default stays
	// standard RRF unless held-out evidence justifies a change.
	LexicalWeight  float64
	SemanticWeight float64
}

func (o HybridOptions) withDefaults() HybridOptions {
	if o.Limit <= 0 {
		o.Limit = 10
	}
	if o.CandidateK <= 0 {
		o.CandidateK = 50
	}
	if o.RRFK <= 0 {
		o.RRFK = 60
	}
	if o.LexicalWeight <= 0 {
		o.LexicalWeight = 1
	}
	if o.SemanticWeight <= 0 {
		o.SemanticWeight = 1
	}
	return o
}

// HybridResult is one fused result with its per-source ranks and RRF score.
type HybridResult struct {
	Memory       asymptoteobserve.LearningMemoryV1
	LexicalRank  int
	SemanticRank int
	Score        float64
}

// SearchHybrid retrieves lexical and semantic candidates independently and fuses
// them with reciprocal rank fusion. When no embedder is configured the semantic
// source is empty and the result is exactly the lexical ranking, so the
// no-model fallback is the existing behaviour. RRF is used rather than adding
// raw scores because BM25-style and cosine scores are not on a common scale.
func (s *Store) SearchHybrid(query Query, opts HybridOptions) ([]HybridResult, error) {
	opts = opts.withDefaults()
	db, err := s.db()
	if err != nil {
		return nil, err
	}

	lexQuery := query
	lexQuery.Limit = opts.CandidateK
	lexQuery.Page = 0
	lexical, err := s.ListMemories(lexQuery)
	if err != nil {
		return nil, err
	}

	var semantic []semanticHit
	if s.embedder != nil {
		qvec := s.embedder.Embed(query.Q)
		if opts.UseANN {
			ids, err := s.semanticSearchANN(db, query, qvec, opts.CandidateK, defaultANNParams())
			if err != nil {
				return nil, err
			}
			semantic, err = s.memoriesByIDs(db, ids)
			if err != nil {
				return nil, err
			}
		} else {
			semantic, err = s.semanticSearchExact(db, query, qvec, opts.CandidateK)
			if err != nil {
				return nil, err
			}
		}
	}

	fused := map[string]*HybridResult{}
	order := make([]string, 0, len(lexical)+len(semantic))
	add := func(m asymptoteobserve.LearningMemoryV1, lexicalRank, semanticRank int) {
		entry, ok := fused[m.ID]
		if !ok {
			entry = &HybridResult{Memory: m, LexicalRank: lexicalRank, SemanticRank: semanticRank}
			fused[m.ID] = entry
			order = append(order, m.ID)
		}
		if lexicalRank > 0 {
			entry.LexicalRank = lexicalRank
			entry.Score += opts.LexicalWeight / (opts.RRFK + float64(lexicalRank))
		}
		if semanticRank > 0 {
			entry.SemanticRank = semanticRank
			entry.Score += opts.SemanticWeight / (opts.RRFK + float64(semanticRank))
		}
	}
	for i, m := range lexical {
		add(m, i+1, 0)
	}
	for i, hit := range semantic {
		add(hit.memory, 0, i+1)
	}

	results := make([]HybridResult, 0, len(order))
	for _, id := range order {
		results = append(results, *fused[id])
	}
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		return results[i].Memory.UpdatedAt > results[j].Memory.UpdatedAt
	})
	if len(results) > opts.Limit {
		results = results[:opts.Limit]
	}
	return results, nil
}

// RebuildEmbeddings computes embeddings for every memory that lacks a current
// one. It is the migration and recovery path for the semantic layer and is
// batched so a large store is not held in memory at once.
func (s *Store) RebuildEmbeddings() error {
	if s.embedder == nil {
		return nil
	}
	db, err := s.db()
	if err != nil {
		return err
	}
	rows, err := db.Query(`SELECT memory_json FROM memories`)
	if err != nil {
		return err
	}
	var batch []asymptoteobserve.LearningMemoryV1
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		defer tx.Rollback()
		for _, m := range batch {
			if err := s.upsertEmbeddingTx(tx, m); err != nil {
				return err
			}
		}
		return tx.Commit()
	}
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			rows.Close()
			return err
		}
		var m asymptoteobserve.LearningMemoryV1
		if err := json.Unmarshal([]byte(raw), &m); err != nil {
			rows.Close()
			return err
		}
		batch = append(batch, m)
		if len(batch) >= 1000 {
			if err := flush(); err != nil {
				rows.Close()
				return err
			}
			batch = batch[:0]
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	return flush()
}
