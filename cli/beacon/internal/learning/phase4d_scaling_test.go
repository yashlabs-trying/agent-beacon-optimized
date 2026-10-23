package learning

// Phase 4D selector scaling test/benchmark. Builds candidate sets of 20/100/500/1000
// memories grouped by shared subjects and compares the indexed (near-linear)
// detector/selector against the trusted O(n^2) path. Also asserts identical
// contradiction counts so the optimization cannot change correctness.

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/asymptote-labs/agent-beacon/pkg/asymptoteobserve"
)

func genCorpus(n int) []asymptoteobserve.LearningMemoryV1 {
	// Realistic diversity: a small number of shared-subject groups (correction
	// chains / conflicts) plus mostly unique-subject memories. Only memories in the
	// same group share >=2 significant terms, so candidate pairs are sparse.
	const groups = 20
	const perGroup = 3
	mems := make([]asymptoteobserve.LearningMemoryV1, 0, n)
	grouped := groups * perGroup
	if grouped > n {
		grouped = n
	}
	for i := 0; i < grouped; i++ {
		g := i / perGroup
		body := fmt.Sprintf("Component%d topic%d item%d.", g, g, i)
		if i%2 == 0 {
			body = fmt.Sprintf("Component%d topic%d must never item%d.", g, g, i)
		}
		mems = append(mems, asymptoteobserve.LearningMemoryV1{
			ID: fmt.Sprintf("m%d", i), Kind: asymptoteobserve.LearningMemoryKindConvention,
			Title: fmt.Sprintf("m%d", i), Body: body,
			Tags: []string{"source:verified"}, Project: asymptoteobserve.LearningProjectV1{ID: "p"},
			Evidence: []asymptoteobserve.LearningEvidenceV1{{TraceID: fmt.Sprintf("t%d", i)}},
		})
	}
	for i := grouped; i < n; i++ {
		mems = append(mems, asymptoteobserve.LearningMemoryV1{
			ID: fmt.Sprintf("m%d", i), Kind: asymptoteobserve.LearningMemoryKindConvention,
			Title: fmt.Sprintf("m%d", i),
			Body:  fmt.Sprintf("Subject%d attribute%d datum%d.", i, i, i),
			Tags:  []string{"source:verified"}, Project: asymptoteobserve.LearningProjectV1{ID: "p"},
			Evidence: []asymptoteobserve.LearningEvidenceV1{{TraceID: fmt.Sprintf("t%d", i)}},
		})
	}
	return mems
}

func TestPhase4DSelectorScaling(t *testing.T) {
	var lines []string
	for _, n := range []int{20, 100, 500, 1000} {
		mems := genCorpus(n)

		start := time.Now()
		naive := detectContradictions(mems, TrustOptions{AllowCrossKind: true, ContradictionTerms: 2})
		naiveMs := float64(time.Since(start).Microseconds()) / 1000.0

		start = time.Now()
		indexed := detectContradictionsIndexed(mems, TrustOptions{AllowCrossKind: true, ContradictionTerms: 2})
		indexedMs := float64(time.Since(start).Microseconds()) / 1000.0

		start = time.Now()
		sel := SelectTrustContext(mems, "component setting topic value",
			TrustContextOptions{EnableSupersession: true, EnableTrust: true, EnableConflict: true, EnableInjectionDefense: true})
		selMs := float64(time.Since(start).Microseconds()) / 1000.0

		if len(naive) != len(indexed) {
			t.Fatalf("n=%d: indexed detector mismatch naive=%d indexed=%d", n, len(naive), len(indexed))
		}
		line := fmt.Sprintf("n=%d naive_detect=%.2fms indexed_detect=%.2fms speedup=%.1fx select_all=%.2fms conflicts=%d kept=%d",
			n, naiveMs, indexedMs, naiveMs/indexedMs, selMs, len(indexed), len(sel.Memories))
		t.Log(line)
		lines = append(lines, line)
	}
	if p := os.Getenv("BEACON_PHASE4D_SCALING_OUT"); p != "" {
		_ = os.WriteFile(p, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
	}
}

func BenchmarkSelectTrustContext1000(b *testing.B) {
	mems := genCorpus(1000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = SelectTrustContext(mems, "component setting topic value",
			TrustContextOptions{EnableSupersession: true, EnableTrust: true, EnableConflict: true, EnableInjectionDefense: true})
	}
}
