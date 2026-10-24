package learning

// Phase 3A Workstream C: contradiction detection over the 24 locked
// contradiction pairs. Opt-in via BEACON_PHASE3_CONTRADICTION=1.
//
// Labels are read from the fixture and never changed. Each pair is evaluated
// in isolation (matching the fixture's pairwise design), then all 48 memories
// are evaluated together to measure cross-pair "cross-talk".

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/asymptote-labs/agent-beacon/pkg/asymptoteobserve"
)

type p3Pair struct {
	ID       string `json:"id"`
	Category string `json:"category"`
	ProjectA string `json:"project_a"`
	ProjectB string `json:"project_b"`
	KindA    string `json:"kind_a"`
	KindB    string `json:"kind_b"`
	VersionA string `json:"version_a"`
	VersionB string `json:"version_b"`
	A        string `json:"a"`
	B        string `json:"b"`
	Conflict bool   `json:"conflict"`
}

type p3ContradictionFixture struct {
	SchemaVersion int      `json:"schema_version"`
	DatasetID     string   `json:"dataset_id"`
	Pairs         []p3Pair `json:"pairs"`
}

func p3LoadContradictionFixture(t *testing.T) p3ContradictionFixture {
	t.Helper()
	path := os.Getenv("BEACON_PHASE3_CONTRADICTION_FIXTURE")
	if path == "" {
		path = filepath.Join("..", "..", "..", "..", "reports", "phase3", "heldout", "contradiction_v1.json")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("locked contradiction fixture not found at %s: %v", path, err)
	}
	var fx p3ContradictionFixture
	if err := json.Unmarshal(raw, &fx); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	if len(fx.Pairs) != 24 {
		t.Fatalf("expected 24 pairs, got %d", len(fx.Pairs))
	}
	return fx
}

func p3PairMemories(p p3Pair) (asymptoteobserve.LearningMemoryV1, asymptoteobserve.LearningMemoryV1) {
	mk := func(id, project, kind, version, body string) asymptoteobserve.LearningMemoryV1 {
		return asymptoteobserve.LearningMemoryV1{
			ID:            id,
			CandidateID:   "cand-" + id,
			Kind:          kind,
			Title:         id,
			Body:          body,
			Applicability: version,
			Project:       asymptoteobserve.LearningProjectV1{ID: project},
			Evidence:      []asymptoteobserve.LearningEvidenceV1{{TraceID: "trace-" + id}},
		}
	}
	return mk(p.ID+"-a", p.ProjectA, p.KindA, p.VersionA, p.A),
		mk(p.ID+"-b", p.ProjectB, p.KindB, p.VersionB, p.B)
}

func TestPhase3AContradictionLocked(t *testing.T) {
	if os.Getenv("BEACON_PHASE3_CONTRADICTION") != "1" {
		t.Skip("set BEACON_PHASE3_CONTRADICTION=1 to run the locked contradiction evaluation")
	}
	fx := p3LoadContradictionFixture(t)
	fmt.Printf("P3A_CONTRADICTION_FIXTURE dataset=%s pairs=%d\n", fx.DatasetID, len(fx.Pairs))

	type counts struct{ tp, fp, fn, tn int }
	byCat := map[string]*counts{}
	total := &counts{}

	for _, p := range fx.Pairs {
		ma, mb := p3PairMemories(p)
		detected := len(detectContradictions([]asymptoteobserve.LearningMemoryV1{ma, mb}, TrustOptions{ContradictionTerms: 2})) > 0
		if byCat[p.Category] == nil {
			byCat[p.Category] = &counts{}
		}
		apply := func(c *counts) {
			switch {
			case detected && p.Conflict:
				c.tp++
			case detected && !p.Conflict:
				c.fp++
			case !detected && p.Conflict:
				c.fn++
			default:
				c.tn++
			}
		}
		apply(byCat[p.Category])
		apply(total)
		verdict := "TN"
		if detected && p.Conflict {
			verdict = "TP"
		} else if detected && !p.Conflict {
			verdict = "FP"
		} else if !detected && p.Conflict {
			verdict = "FN"
		}
		fmt.Printf("P3A_CONTRADICTION pair=%s category=%s label_conflict=%v detected=%v verdict=%s\n",
			p.ID, p.Category, p.Conflict, detected, verdict)
	}

	report := func(name string, c *counts) {
		prec, rec, f1 := 0.0, 0.0, 0.0
		if c.tp+c.fp > 0 {
			prec = float64(c.tp) / float64(c.tp+c.fp)
		}
		if c.tp+c.fn > 0 {
			rec = float64(c.tp) / float64(c.tp+c.fn)
		}
		if prec+rec > 0 {
			f1 = 2 * prec * rec / (prec + rec)
		}
		fmt.Printf("P3A_CONTRADICTION category=%s tp=%d fp=%d fn=%d tn=%d precision=%.4f recall=%.4f f1=%.4f\n",
			name, c.tp, c.fp, c.fn, c.tn, prec, rec, f1)
	}
	for _, cat := range []string{"same_kind", "cross_kind", "distinct_version", "cross_project", "same_polarity", "unrelated"} {
		if byCat[cat] != nil {
			report(cat, byCat[cat])
		}
	}
	report("ALL", total)

	// Cross-talk: evaluate all 48 memories at once and count detections that do
	// not correspond to a labeled conflict pair.
	var all []asymptoteobserve.LearningMemoryV1
	expected := map[string]bool{}
	for _, p := range fx.Pairs {
		ma, mb := p3PairMemories(p)
		all = append(all, ma, mb)
		if p.Conflict {
			expected[ma.ID+"|"+mb.ID] = true
		}
	}
	detectedAll := detectContradictions(all, TrustOptions{ContradictionTerms: 2})
	extra, matched := 0, 0
	for _, c := range detectedAll {
		key := c.A.ID + "|" + c.B.ID
		if expected[key] {
			matched++
		} else {
			extra++
			fmt.Printf("P3A_CONTRADICTION crosstalk a=%s b=%s shared=%v\n", c.A.ID, c.B.ID, c.SharedTerm)
		}
	}
	fmt.Printf("P3A_CONTRADICTION crosstalk_summary expected_conflicts=%d detected_total=%d matched=%d extra_crosstalk=%d\n",
		len(expected), len(detectedAll), matched, extra)
}
