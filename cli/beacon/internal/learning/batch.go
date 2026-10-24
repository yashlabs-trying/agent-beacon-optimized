package learning

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/asymptote-labs/agent-beacon/pkg/asymptoteobserve"
)

// MaxBatchSize bounds a single batch so a caller cannot buffer an unbounded
// number of records in one transaction.
const MaxBatchSize = 1000

// PutMemories inserts or updates a batch of memories in ONE immediate
// transaction, so the batch pays a single commit (and, under WAL, a single
// fsync) instead of one per record. It is the high-throughput ingestion path;
// PutMemory remains the single-record path and behaves identically for one item.
//
// The batch is atomic: every record commits or none does. Authorization is
// enforced for every record's project before any write, so a batch that names a
// project the store is not authorized for is rejected in full. Records are
// written in order; deterministic ids make a retry idempotent.
func (s *Store) PutMemories(ms []asymptoteobserve.LearningMemoryV1) error {
	return s.PutMemoriesContext(context.Background(), ms)
}

// PutMemoriesContext is PutMemories with cancellation. The context is checked
// before the transaction and between records; cancelling rolls the whole batch
// back.
func (s *Store) PutMemoriesContext(ctx context.Context, ms []asymptoteobserve.LearningMemoryV1) error {
	if len(ms) == 0 {
		return nil
	}
	if len(ms) > MaxBatchSize {
		return fmt.Errorf("batch too large: %d records (max %d)", len(ms), MaxBatchSize)
	}
	type item struct {
		m    asymptoteobserve.LearningMemoryV1
		data []byte
	}
	items := make([]item, 0, len(ms))
	for _, m := range ms {
		if err := s.authorizeLifecycle(m.Project.ID); err != nil {
			return err
		}
		normalized := normalizeMemoryForWrite(m)
		data, err := json.Marshal(normalized)
		if err != nil {
			return err
		}
		items = append(items, item{normalized, data})
	}
	db, err := s.db()
	if err != nil {
		return err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return s.busyRetryContext(ctx, func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		faultPoint("tx_begin_pre")
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("begin batch: %w", err)
		}
		faultPoint("tx_begin_post")
		defer tx.Rollback()
		indexHealthy := true
		for _, it := range items {
			if err := ctx.Err(); err != nil {
				return err
			}
			healthy, err := s.writeMemoryTx(tx, it.m, it.data)
			if err != nil {
				return err
			}
			if !healthy {
				indexHealthy = false
			}
		}
		faultPoint("pre_commit")
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit batch: %w", err)
		}
		faultPoint("post_commit")
		if !indexHealthy {
			s.markIndexUnhealthy(db)
		}
		return nil
	})
}
