package learning

// Phase 4C Benchmark D: bounded memory-reliability test. As corrections accumulate
// over simulated sessions, the context-selection layer must retain the currently
// applicable fact and drop the superseded chain, without letting an untrusted
// correction override verified knowledge.

import (
	"fmt"
	"testing"
	"time"

	"github.com/asymptote-labs/agent-beacon/pkg/asymptoteobserve"
)

func histMem(id, body, ts, source string) asymptoteobserve.LearningMemoryV1 {
	return asymptoteobserve.LearningMemoryV1{
		ID: id, Kind: asymptoteobserve.LearningMemoryKindConvention, Title: id, Body: body,
		Tags:      []string{"source:" + source},
		Project:   asymptoteobserve.LearningProjectV1{ID: "hist"},
		Evidence:  []asymptoteobserve.LearningEvidenceV1{{TraceID: "t-" + id}},
		UpdatedAt: ts, CreatedAt: ts,
	}
}

func TestPhase4CMemoryHistory(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, n := range []int{10, 50, 100} {
		mems := []asymptoteobserve.LearningMemoryV1{
			histMem("fact-0", "The service port is 8080.", base.Format(time.RFC3339Nano), "verified"),
		}
		for i := 1; i < n; i++ {
			mems = append(mems, histMem(
				fmt.Sprintf("fact-%d", i),
				fmt.Sprintf("The service port is now %d.", 8080+i),
				base.Add(time.Duration(i)*24*time.Hour).Format(time.RFC3339Nano), "verified"))
		}
		start := time.Now()
		sel := SelectTrustContext(mems, "service port", TrustContextOptions{EnableSupersession: true})
		elapsed := time.Since(start)
		if len(sel.Memories) != 1 || sel.Memories[0].ID != fmt.Sprintf("fact-%d", n-1) {
			t.Fatalf("n=%d: expected only the latest fact, got %d memories", n, len(sel.Memories))
		}
		t.Logf("HISTORY n=%d kept=%s superseded=%d elapsed=%s", n, sel.Memories[0].ID, len(sel.Signals.SupersededIDs), elapsed)
	}

	// Untrusted correction must not override verified knowledge.
	mems := []asymptoteobserve.LearningMemoryV1{
		histMem("v", "The service port is 8080.", base.Format(time.RFC3339Nano), "verified"),
		histMem("u", "The service port is now 9.", base.Add(48*time.Hour).Format(time.RFC3339Nano), "untrusted"),
	}
	sel := SelectTrustContext(mems, "service port", TrustContextOptions{EnableSupersession: true, EnableTrust: true})
	if len(sel.Memories) != 1 || sel.Memories[0].ID != "v" {
		t.Fatalf("untrusted correction overrode verified memory: got %d", len(sel.Memories))
	}
	t.Logf("HISTORY untrusted-correction kept=%s", sel.Memories[0].ID)
}
