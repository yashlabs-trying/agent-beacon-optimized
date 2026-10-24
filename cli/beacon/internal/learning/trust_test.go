package learning

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/asymptote-labs/agent-beacon/pkg/asymptoteobserve"
)

func TestTrustEvidenceStatus(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	recent := now.Add(-10 * 24 * time.Hour).Format(time.RFC3339Nano)
	old := now.Add(-400 * 24 * time.Hour).Format(time.RFC3339Nano)
	project := asymptoteobserve.LearningProjectV1{ID: "proj-a"}

	memories := []asymptoteobserve.LearningMemoryV1{
		{ID: "verified", Kind: "k", Title: "t", Body: "has evidence", Project: project,
			UpdatedAt: recent, Evidence: []asymptoteobserve.LearningEvidenceV1{{TraceID: "tr-1"}}},
		{ID: "unverified", Kind: "k", Title: "t", Body: "no evidence", Project: project, UpdatedAt: recent},
		{ID: "empty-source", Kind: "k", Title: "t", Body: "blank trace", Project: project,
			UpdatedAt: recent, Evidence: []asymptoteobserve.LearningEvidenceV1{{TraceID: "  "}}},
		{ID: "stale", Kind: "k", Title: "t", Body: "old but evidenced", Project: project,
			UpdatedAt: old, Evidence: []asymptoteobserve.LearningEvidenceV1{{TraceID: "tr-2"}}},
		{ID: "superseded", Kind: "k", Title: "t", Body: "replaced", Project: project,
			UpdatedAt: recent, SupersededBy: "verified",
			Evidence: []asymptoteobserve.LearningEvidenceV1{{TraceID: "tr-3"}}},
	}
	got := AnnotateTrust(memories, TrustOptions{Now: now, MaxAgeDays: 180})
	want := map[string]TrustStatus{
		"verified":     TrustVerified,
		"unverified":   TrustUnverified,
		"empty-source": TrustUnverified,
		"stale":        TrustStale,
		"superseded":   TrustVerified, // approval/supersession is separate from evidence status
	}
	for _, tm := range got {
		if want[tm.Memory.ID] != tm.Status {
			t.Errorf("%s: status=%s want=%s (reasons=%v)", tm.Memory.ID, tm.Status, want[tm.Memory.ID], tm.Reasons)
		}
	}
}

// TestTrustContradictionPrecisionRecall measures deterministic contradiction
// detection against labeled pairs. True pairs share >=2 significant terms with
// opposite polarity; decoy pairs share terms with the same polarity. Each pair
// uses its own kind so cross-pair detection cannot occur.
func TestTrustContradictionPrecisionRecall(t *testing.T) {
	project := asymptoteobserve.LearningProjectV1{ID: "proj-a"}
	type pair struct {
		id, kind string
		a, b     string
		conflict bool
	}
	pairs := []pair{
		{"p1", "k1", "Always pin the dependency versions before building.", "Do not pin dependency versions when building locally.", true},
		{"p2", "k2", "Always cancel the request context to avoid goroutine leaks.", "Never cancel the request context while a stream is active.", true},
		{"p3", "k3", "Prefer retrying the fetch after a transient error.", "Avoid retrying the fetch after a transient error.", true},
		{"p4", "k4", "Ensure the cache is invalidated after a config change.", "Do not invalidate the cache after a config change.", true},
		{"d1", "d1", "Always run the formatter before committing.", "Always run the linter before committing.", false},
		{"d2", "d2", "Prefer small pull requests for review.", "Prefer small commits for review.", false},
		{"d3", "d3", "Never force push to a shared branch.", "Never rewrite a shared branch history.", false},
		{"d4", "d4", "Ensure tests run in parallel.", "Ensure tests run in isolation.", false},
	}

	var memories []asymptoteobserve.LearningMemoryV1
	labeled := map[string]bool{}
	for _, p := range pairs {
		aid, bid := p.id+"-a", p.id+"-b"
		memories = append(memories,
			asymptoteobserve.LearningMemoryV1{ID: aid, Kind: p.kind, Title: "a", Body: p.a, Project: project},
			asymptoteobserve.LearningMemoryV1{ID: bid, Kind: p.kind, Title: "b", Body: p.b, Project: project},
		)
		key := aid + "|" + bid
		if p.conflict {
			labeled[key] = true
		}
	}

	detected := detectContradictions(memories, TrustOptions{})
	detectedSet := map[string]bool{}
	for _, c := range detected {
		detectedSet[c.A.ID+"|"+c.B.ID] = true
	}

	tp, fp, fn := 0, 0, 0
	for key := range detectedSet {
		if labeled[key] {
			tp++
		} else {
			fp++
		}
	}
	for key := range labeled {
		if !detectedSet[key] {
			fn++
		}
	}
	precision, recall := 0.0, 0.0
	if tp+fp > 0 {
		precision = float64(tp) / float64(tp+fp)
	}
	if tp+fn > 0 {
		recall = float64(tp) / float64(tp+fn)
	}
	fmt.Printf("TRUST contradiction true_pairs=%d detected=%d tp=%d fp=%d fn=%d precision=%.3f recall=%.3f\n",
		len(labeled), len(detectedSet), tp, fp, fn, precision, recall)
	if precision < 1.0 || recall < 1.0 {
		t.Fatalf("deterministic contradiction detection not exact on the labeled set: precision=%.3f recall=%.3f", precision, recall)
	}
}

// TestTrustNoCrossScopeLeakage verifies that a query for one project never
// returns another project's memories, with or without the semantic layer.
func TestTrustNoCrossScopeLeakage(t *testing.T) {
	store := OpenSemantic(filepath.Join(t.TempDir(), "memory.db"), NewHashedEmbedder())
	a := asymptoteobserve.LearningProjectV1{ID: "proj-a"}
	b := asymptoteobserve.LearningProjectV1{ID: "proj-b"}
	for i := 0; i < 30; i++ {
		if err := store.PutMemory(asymptoteobserve.LearningMemoryV1{
			ID: fmt.Sprintf("a-%02d", i), CandidateID: fmt.Sprintf("ca-%02d", i),
			Kind: asymptoteobserve.LearningMemoryKindConvention, Title: "note",
			Body: "the shared token appears here", Project: a,
			Evidence: []asymptoteobserve.LearningEvidenceV1{{TraceID: "tr"}},
		}); err != nil {
			t.Fatal(err)
		}
		if err := store.PutMemory(asymptoteobserve.LearningMemoryV1{
			ID: fmt.Sprintf("b-%02d", i), CandidateID: fmt.Sprintf("cb-%02d", i),
			Kind: asymptoteobserve.LearningMemoryKindConvention, Title: "note",
			Body: "the shared token appears here", Project: b,
			Evidence: []asymptoteobserve.LearningEvidenceV1{{TraceID: "tr"}},
		}); err != nil {
			t.Fatal(err)
		}
	}
	for _, q := range []Query{{ProjectID: "proj-a", Q: "shared token"}, {ProjectID: "proj-b", Q: "shared token"}} {
		lex, err := store.ListMemories(Query{ProjectID: q.ProjectID, Q: q.Q, Limit: 50})
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range lex {
			if m.Project.ID != q.ProjectID {
				t.Fatalf("lexical cross-scope leak: query %s returned %s", q.ProjectID, m.Project.ID)
			}
		}
		hyb, err := store.SearchHybrid(q, HybridOptions{Limit: 50, CandidateK: 100})
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range hyb {
			if r.Memory.Project.ID != q.ProjectID {
				t.Fatalf("hybrid cross-scope leak: query %s returned %s", q.ProjectID, r.Memory.Project.ID)
			}
		}
	}
}
