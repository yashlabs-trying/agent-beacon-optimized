package learning

import (
	"context"
	"strings"
	"time"
)

// Bounded cross-process BUSY retry. SQLite serializes writers across processes;
// busy_timeout(30000) makes a competing writer wait, but if the wait is exceeded
// the driver returns SQLITE_BUSY. Rather than surface that to the caller, a write
// transaction is retried a bounded number of times with a short backoff.
//
// Retrying is safe because a BUSY failure means the transaction never committed:
// the deferred rollback leaves no partial state. The write paths use deterministic
// ids and conditional state updates, so a retry cannot duplicate a row or replay
// a committed transition.

const (
	busyRetryMaxAttempts = 5
	busyRetryBaseBackoff = 20 * time.Millisecond
)

// busyRetry runs fn, retrying on a SQLITE_BUSY/LOCKED error with bounded attempts
// and linear backoff. It never retries indefinitely.
func (s *Store) busyRetry(fn func() error) error {
	return s.busyRetryContext(context.Background(), fn)
}

// busyRetryContext is busyRetry that stops as soon as ctx is canceled or its
// deadline expires, so a canceled writer does not keep waiting on a lock.
func (s *Store) busyRetryContext(ctx context.Context, fn func() error) error {
	var err error
	for attempt := 0; attempt < busyRetryMaxAttempts; attempt++ {
		if cerr := ctx.Err(); cerr != nil {
			return cerr
		}
		err = fn()
		if err == nil || !isBusyError(err) {
			return err
		}
		if attempt < busyRetryMaxAttempts-1 {
			s.busyRetries.Add(1)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(attempt+1) * busyRetryBaseBackoff):
			}
		}
	}
	s.busyExhausted.Add(1)
	return err
}

// BusyRetryStats reports the number of retries performed and the number that
// exhausted the retry budget, for observability and tests.
func (s *Store) BusyRetryStats() (retries, exhausted int64) {
	return s.busyRetries.Load(), s.busyExhausted.Load()
}

func isBusyError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "database is locked") ||
		strings.Contains(msg, "database table is locked") ||
		strings.Contains(msg, "sqlite_busy") ||
		strings.Contains(msg, "sqlite_locked")
}
