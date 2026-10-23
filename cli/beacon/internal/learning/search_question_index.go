package learning

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/asymptote-labs/agent-beacon/pkg/asymptoteobserve"
)

// First use of optional semantic search may encounter memories written before
// the embedder was enabled. Catch up only the requested project, and cap work
// so one agent question cannot unexpectedly embed a very large store.
func (s *Store) ensureQuestionEmbeddings(db *sql.DB, projectID string) error {
	const maxOnDemand = 1000
	const missing = `FROM memories m LEFT JOIN memory_embeddings e ON e.memory_id = m.id
		WHERE m.project_id = ? AND (m.superseded_by IS NULL OR m.superseded_by = '')
		AND (e.memory_id IS NULL OR e.model != ? OR e.dim != ?)`
	args := []any{projectID, s.embedder.Model(), s.embedder.Dim()}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) `+missing, args...).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		return nil
	}
	if count > maxOnDemand {
		return fmt.Errorf("semantic index has %d missing or outdated embeddings; run RebuildEmbeddings before question-aware semantic search", count)
	}
	rows, err := db.Query(`SELECT m.memory_json `+missing, args...)
	if err != nil {
		return err
	}
	memories := make([]asymptoteobserve.LearningMemoryV1, 0, count)
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
		memories = append(memories, m)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, m := range memories {
		if err := s.upsertEmbeddingTx(tx, m); err != nil {
			return err
		}
	}
	return tx.Commit()
}
