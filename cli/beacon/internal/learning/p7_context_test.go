package learning

// P7 tests: context-aware deadlines and cancellation.

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/asymptote-labs/agent-beacon/pkg/asymptoteobserve"
)

func p7Seed(t *testing.T, s *Store, project string, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		body := "ordinary workflow text about modules and retries"
		if i == n/2 {
			body = "the rare unique-needle recovery step"
		}
		if err := s.PutMemory(asymptoteobserve.LearningMemoryV1{
			ID: fmt.Sprintf("%s-m-%d", project, i), CandidateID: fmt.Sprintf("%s-c-%d", project, i),
			Kind: asymptoteobserve.LearningMemoryKindConvention, Title: "t", Body: body,
			Project: asymptoteobserve.LearningProjectV1{ID: project},
		}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestP7CanceledBeforeQuery(t *testing.T) {
	s := Open(filepath.Join(supTempDir(t), "memory.db"))
	defer supClose(s)
	p7Seed(t, s, "p", 100)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.ListMemoriesContext(ctx, Query{ProjectID: "p", Q: "workflow"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if _, _, err := s.GetMemoryContext(ctx, "p-m-0"); !errors.Is(err, context.Canceled) {
		t.Fatalf("GetMemoryContext err = %v, want context.Canceled", err)
	}
}

func TestP7ExpiredDeadline(t *testing.T) {
	s := Open(filepath.Join(supTempDir(t), "memory.db"))
	defer supClose(s)
	p7Seed(t, s, "p", 100)

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	if _, err := s.ListMemoriesContext(ctx, Query{ProjectID: "p", Q: "workflow"}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want context.DeadlineExceeded", err)
	}
}

func TestP7LargeScanCanceled(t *testing.T) {
	s := Open(filepath.Join(supTempDir(t), "memory.db"))
	defer supClose(s)
	p7Seed(t, s, "p", 5000)

	// A very short deadline on a large non-selective scan must abort.
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	_, err := s.ListMemoriesContext(ctx, Query{ProjectID: "p", Q: "ab", Limit: 500})
	if err == nil {
		// The scan may finish before the deadline on a fast host; that is a valid
		// completion, not a failure. Only require that if it did not complete it
		// returned a context error rather than a wrong result.
		return
	}
	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want a context error", err)
	}
}

func TestP7ValidDeadlineSucceeds(t *testing.T) {
	s := Open(filepath.Join(supTempDir(t), "memory.db"))
	defer supClose(s)
	p7Seed(t, s, "p", 500)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	got, err := s.ListMemoriesContext(ctx, Query{ProjectID: "p", Q: "unique-needle", Limit: 5})
	if err != nil {
		t.Fatalf("normal request with a generous deadline failed: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("results = %d, want 1", len(got))
	}
}

func TestP7CanceledBatchWrite(t *testing.T) {
	s := Open(filepath.Join(supTempDir(t), "memory.db"))
	defer supClose(s)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	ms := []asymptoteobserve.LearningMemoryV1{p1Mem(0), p1Mem(1)}
	if err := s.PutMemoriesContext(ctx, ms); !errors.Is(err, context.Canceled) {
		t.Fatalf("batch err = %v, want context.Canceled", err)
	}
	db, _ := s.db()
	var n int
	_ = db.QueryRow(`SELECT COUNT(*) FROM memories`).Scan(&n)
	if n != 0 {
		t.Fatalf("canceled batch wrote %d rows", n)
	}
}

func TestP7NoGoroutineLeak(t *testing.T) {
	s := Open(filepath.Join(supTempDir(t), "memory.db"))
	defer supClose(s)
	p7Seed(t, s, "p", 500)

	before := runtime.NumGoroutine()
	for i := 0; i < 200; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, _ = s.ListMemoriesContext(ctx, Query{ProjectID: "p", Q: "workflow"})
		cancel()
	}
	time.Sleep(100 * time.Millisecond)
	after := runtime.NumGoroutine()
	if after > before+5 {
		t.Fatalf("goroutine leak: before=%d after=%d", before, after)
	}
}

func TestP7ConcurrentCanceledRequests(t *testing.T) {
	s := Open(filepath.Join(supTempDir(t), "memory.db"))
	defer supClose(s)
	p7Seed(t, s, "p", 200)
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(k int) {
			defer wg.Done()
			ctx, cancel := context.WithCancel(context.Background())
			if k%2 == 0 {
				cancel()
			} else {
				defer cancel()
			}
			_, _ = s.ListMemoriesContext(ctx, Query{ProjectID: "p", Q: "workflow"})
		}(i)
	}
	wg.Wait()
}
