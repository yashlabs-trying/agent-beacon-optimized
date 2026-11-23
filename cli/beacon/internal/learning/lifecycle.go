package learning

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/asymptote-labs/agent-beacon/pkg/asymptoteobserve"
)

// ErrCandidateNotReviewable is returned when a candidate transition is attempted
// but the stored candidate is no longer in the expected state (already approved,
// rejected, or superseded, possibly by a concurrent reviewer). The transition is
// rejected without writing anything.
var ErrCandidateNotReviewable = errors.New("candidate is no longer reviewable")

// normalizeMemoryForWrite fills the fields PutMemory and the atomic lifecycle
// write require before persisting a memory.
func normalizeMemoryForWrite(m asymptoteobserve.LearningMemoryV1) asymptoteobserve.LearningMemoryV1 {
	if m.SchemaVersion == "" {
		m.SchemaVersion = asymptoteobserve.LearningSchemaVersion
	}
	now := nowString()
	if m.CreatedAt == "" {
		m.CreatedAt = now
	}
	m.UpdatedAt = now
	if m.ID == "" {
		m.ID = MemoryID(m)
	}
	return m
}

// normalizeCandidateForWrite fills the fields a candidate write requires.
func normalizeCandidateForWrite(c asymptoteobserve.LearningCandidateV1) asymptoteobserve.LearningCandidateV1 {
	if c.SchemaVersion == "" {
		c.SchemaVersion = asymptoteobserve.LearningSchemaVersion
	}
	now := nowString()
	if c.CreatedAt == "" {
		c.CreatedAt = now
	}
	c.UpdatedAt = now
	if c.State == "" {
		c.State = asymptoteobserve.LearningCandidateStateCandidate
	}
	if c.ID == "" {
		c.ID = CandidateID(c)
	}
	return c
}

// writeMemoryTx writes the authoritative memory row and its derived state
// (FTS index, cache generation, embeddings) inside an existing transaction. It
// returns whether the derived index was refreshed; a false result means the
// caller should mark the index unhealthy after commit. It never fails the
// authoritative write because of a derived-index problem.
func (s *Store) writeMemoryTx(tx *sql.Tx, m asymptoteobserve.LearningMemoryV1, data []byte) (bool, error) {
	if _, err := tx.Exec(`INSERT INTO memories (id, candidate_id, project_id, kind, title, created_at, updated_at, superseded_by, memory_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET title = excluded.title, updated_at = excluded.updated_at, superseded_by = excluded.superseded_by, memory_json = excluded.memory_json`,
		m.ID, m.CandidateID, m.Project.ID, m.Kind, m.Title, m.CreatedAt, m.UpdatedAt, m.SupersededBy, string(data)); err != nil {
		return false, fmt.Errorf("insert memory: %w", err)
	}
	if s.writeCheckpoint != nil {
		s.writeCheckpoint("after-authoritative")
	}
	faultPoint("memory_insert")
	// Refresh the derived index row in the same transaction when the index is
	// known-good, and advance the durable generation either way so every process
	// sees that cached results for this store are now stale.
	indexHealthy := indexReadyTx(tx)
	if indexHealthy {
		if err := upsertMemoryIndex(tx, m); err != nil {
			// A derived-index problem must never fail the authoritative write.
			// Mark the index not-ready so the next search rebuilds it, and let
			// the authoritative row and the generation advance commit normally.
			indexHealthy = false
			if _, markErr := tx.Exec(`UPDATE memory_index_meta SET value = '0' WHERE key = 'index_ready'`); markErr != nil {
				return false, fmt.Errorf("index memory: %w", err)
			}
		}
	}
	if s.writeCheckpoint != nil {
		s.writeCheckpoint("after-index")
	}
	faultPoint("fts_update")
	if _, err := tx.Exec(`UPDATE memory_index_meta SET value = CAST(value AS INTEGER) + 1 WHERE key = 'generation'`); err != nil {
		return false, fmt.Errorf("bump generation: %w", err)
	}
	if s.writeCheckpoint != nil {
		s.writeCheckpoint("after-generation")
	}
	faultPoint("generation_update")
	if s.embedder != nil {
		// Embeddings are derived state, updated incrementally in the same
		// transaction. A failure here must never fail the authoritative write;
		// RebuildEmbeddings can repair it later.
		_ = s.upsertEmbeddingTx(tx, m)
	} else {
		// A lexical writer may update a memory after another process embedded
		// it. Invalidate that derived row so the next semantic query rebuilds
		// the current content instead of serving a stale vector.
		_, _ = tx.Exec(`DELETE FROM memory_embeddings WHERE memory_id = ?`, m.ID)
	}
	return indexHealthy, nil
}

// putCandidateTx writes a candidate row inside an existing transaction.
func putCandidateTx(tx *sql.Tx, c asymptoteobserve.LearningCandidateV1) error {
	data, err := json.Marshal(c)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT INTO candidates (id, project_id, state, kind, title, source_evaluation_id, memory_id, created_at, updated_at, candidate_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET state = excluded.state, kind = excluded.kind, title = excluded.title, memory_id = excluded.memory_id, updated_at = excluded.updated_at, candidate_json = excluded.candidate_json`,
		c.ID, c.Project.ID, c.State, c.Kind, c.Title, c.SourceEvaluationID, c.MemoryID, c.CreatedAt, c.UpdatedAt, string(data))
	return err
}

// TransitionOptions configures CommitCandidateTransition.
type TransitionOptions struct {
	// ExpectStates is the set of candidate states the transition is legal from.
	// An empty set is rejected so a caller can never authorize every state.
	ExpectStates []string
	// TombstoneBy, when non-empty, marks the candidate's current memory (read
	// inside the transaction, so a concurrent approval cannot race past it) as
	// superseded by this replacement memory ID.
	TombstoneBy string
	// PreserveMemoryID keeps the candidate's committed memory_id instead of the
	// value carried by the passed candidate. Supersession does not change which
	// memory the candidate points at, so it must not clobber one created by a
	// concurrent approval.
	PreserveMemoryID bool
	// AuditEvent, when non-empty, writes an audit row in the same transaction as
	// the transition. A failure to write it rolls the transition back, so a
	// success record can never outlive a rolled-back mutation.
	AuditEvent     string
	AuditActor     string
	AuditRequestID string
}

// CommitCandidateTransition applies a candidate review transition and, when a
// memory write is requested, the matching authoritative memory write in a single
// immediate transaction. The candidate row must still be in one of
// opts.ExpectStates; otherwise the whole transaction is rolled back and
// ErrCandidateNotReviewable is returned.
//
// ExpectStates is a set, not a single value, because a transition can be legal
// from more than one state: supersession is valid from "candidate" (never
// approved) and from "approved" (an approved candidate whose memory must be
// tombstoned). Supersession passes TombstoneBy and PreserveMemoryID so the
// tombstone targets whatever memory is committed at transaction time, which
// closes the approval/supersession race.
func (s *Store) CommitCandidateTransition(candidate asymptoteobserve.LearningCandidateV1, memory *asymptoteobserve.LearningMemoryV1, opts TransitionOptions) error {
	if len(opts.ExpectStates) == 0 {
		return fmt.Errorf("commit transition: no expected state")
	}
	candidate = normalizeCandidateForWrite(candidate)
	var mem asymptoteobserve.LearningMemoryV1
	var memData []byte
	if memory != nil {
		mem = normalizeMemoryForWrite(*memory)
		var err error
		if memData, err = json.Marshal(mem); err != nil {
			return err
		}
	}
	db, err := s.db()
	if err != nil {
		return err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return s.busyRetry(func() error {
		faultPoint("tx_begin_pre")
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("begin transition: %w", err)
		}
		faultPoint("tx_begin_post")
		defer tx.Rollback()
		if s.writeCheckpoint != nil {
			s.writeCheckpoint("after-begin")
		}
		// Read the candidate's committed memory_id inside the transaction so a
		// concurrent approval that committed first is not lost.
		if opts.PreserveMemoryID {
			var cur sql.NullString
			if err := tx.QueryRow(`SELECT memory_id FROM candidates WHERE id = ?`, candidate.ID).Scan(&cur); err == nil {
				candidate.MemoryID = cur.String
			}
		}
		candidateData, err := json.Marshal(candidate)
		if err != nil {
			return err
		}
		// Conditional state check: the update only matches a row still in one of
		// expectStates, so two concurrent reviewers cannot both transition it.
		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(opts.ExpectStates)), ",")
		args := []any{candidate.State, candidate.Kind, candidate.Title, candidate.MemoryID, candidate.UpdatedAt, string(candidateData), candidate.ID}
		for _, st := range opts.ExpectStates {
			args = append(args, st)
		}
		res, err := tx.Exec(`UPDATE candidates SET state = ?, kind = ?, title = ?, memory_id = ?, updated_at = ?, candidate_json = ?
		WHERE id = ? AND state IN (`+placeholders+`)`, args...)
		if err != nil {
			return fmt.Errorf("update candidate: %w", err)
		}
		affected, err := res.RowsAffected()
		if err != nil {
			return err
		}
	if affected != 1 {
		return ErrCandidateNotReviewable
	}
	if opts.AuditEvent != "" {
		if err := appendAuditTx(tx, AuditEvent{
			Type: opts.AuditEvent, Actor: opts.AuditActor, Project: candidate.Project.ID,
			Target: candidate.ID, Outcome: "success", RequestID: opts.AuditRequestID,
		}); err != nil {
			return fmt.Errorf("audit: %w", err)
		}
	}
	indexHealthy := true
		switch {
		case opts.TombstoneBy != "" && candidate.MemoryID != "":
			var raw string
			if err := tx.QueryRow(`SELECT memory_json FROM memories WHERE id = ?`, candidate.MemoryID).Scan(&raw); err == nil {
				var old asymptoteobserve.LearningMemoryV1
				if err := json.Unmarshal([]byte(raw), &old); err == nil {
					old.SupersededBy = opts.TombstoneBy
					old = normalizeMemoryForWrite(old)
					data, err := json.Marshal(old)
					if err != nil {
						return err
					}
					if indexHealthy, err = s.writeMemoryTx(tx, old, data); err != nil {
						return err
					}
				}
			}
		case memory != nil:
			if indexHealthy, err = s.writeMemoryTx(tx, mem, memData); err != nil {
				return err
			}
		}
		if s.writeCheckpoint != nil {
			s.writeCheckpoint("before-commit")
		}
		faultPoint("pre_commit")
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit transition: %w", err)
		}
		if s.writeCheckpoint != nil {
			s.writeCheckpoint("after-commit")
		}
		faultPoint("post_commit")
		if !indexHealthy {
			s.markIndexUnhealthy(db)
		}
		return nil
	})
}
