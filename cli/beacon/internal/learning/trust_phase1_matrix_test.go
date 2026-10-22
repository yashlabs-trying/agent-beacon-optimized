package learning

import (
	"fmt"
	"testing"
	"time"

	"github.com/asymptote-labs/agent-beacon/pkg/asymptoteobserve"
)

// Fifty labeled cases: 20 evidence-status cases and 30 contradiction pairs.
func TestTrustPhase1LabeledScenarios(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		want bool
	}{
		{"software version 1", "software version 2", true},
		{"software version 1", "software version 1.0", false},
		{"software version 1 or 2", "software version 2", false},
		{"software version 1", "all versions", false},
	} {
		if got := distinctExplicitVersions(tc.a, tc.b); got != tc.want {
			t.Fatalf("version applicability %q / %q: got %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
	now := time.Date(2026, 10, 22, 0, 0, 0, 0, time.UTC)
	recent := now.Add(-24 * time.Hour).Format(time.RFC3339Nano)
	old := now.Add(-400 * 24 * time.Hour).Format(time.RFC3339Nano)
	for i := 0; i < 20; i++ {
		m := asymptoteobserve.LearningMemoryV1{
			ID: fmt.Sprintf("status-%02d", i), Kind: "k", Title: "status",
			Body: "a documented convention", Project: asymptoteobserve.LearningProjectV1{ID: "p"},
			UpdatedAt: recent,
		}
		want := TrustVerified
		switch i % 5 {
		case 0:
			m.Evidence = []asymptoteobserve.LearningEvidenceV1{{TraceID: "trace-1"}}
		case 1:
			m.Evidence = []asymptoteobserve.LearningEvidenceV1{{TraceID: "   "}}
			want = TrustUnverified
		case 2:
			want = TrustUnverified
		case 3:
			m.Evidence = []asymptoteobserve.LearningEvidenceV1{{TraceID: "trace-1"}}
			m.UpdatedAt = old
			want = TrustStale
		case 4:
			m.Evidence = []asymptoteobserve.LearningEvidenceV1{{TraceID: "trace-1"}}
			m.SupersededBy = "replacement"
		}
		got := AnnotateTrust([]asymptoteobserve.LearningMemoryV1{m}, TrustOptions{Now: now, MaxAgeDays: 180})
		if len(got) != 1 || got[0].Status != want {
			t.Fatalf("status scenario %d: got %v, want %s", i, got, want)
		}
	}

	actions := []string{
		"refresh cache", "rotate credentials", "validate schema", "cancel context", "rebuild index",
		"pin dependencies", "verify checksums", "retry downloads", "limit tokens", "record provenance",
	}
	tp, fp, fn := 0, 0, 0
	for i := 0; i < 30; i++ {
		action := actions[i%len(actions)]
		a := asymptoteobserve.LearningMemoryV1{
			ID: "a", Kind: "k", Body: "Always " + action + " before deployment.",
			Project: asymptoteobserve.LearningProjectV1{ID: "project-a"},
		}
		b := a
		b.ID = "b"
		b.Body = "Never " + action + " before deployment."
		if i == 0 {
			a.Applicability = "software version 1"
			b.Applicability = "software version 1.0"
		}
		wantConflict := i < 10
		switch {
		case i >= 10 && i < 15:
			b.Body = "Ensure you " + action + " before deployment."
		case i >= 15 && i < 20:
			b.Project.ID = "project-b"
		case i >= 20 && i < 25:
			b.Kind = "other-kind"
		case i >= 25:
			a.Applicability = "software version 1"
			b.Applicability = "software version 2"
		}
		got := len(detectContradictions([]asymptoteobserve.LearningMemoryV1{a, b}, TrustOptions{})) > 0
		if got && wantConflict {
			tp++
		} else if got {
			fp++
		} else if wantConflict {
			fn++
		}
	}
	fmt.Printf("TRUST_MATRIX status_cases=20 pair_cases=30 tp=%d fp=%d fn=%d precision=%.3f recall=%.3f\n",
		tp, fp, fn, float64(tp)/float64(tp+fp), float64(tp)/float64(tp+fn))
	if fp != 0 || fn != 0 {
		t.Fatalf("contradiction labels disagreed: fp=%d fn=%d", fp, fn)
	}
}
