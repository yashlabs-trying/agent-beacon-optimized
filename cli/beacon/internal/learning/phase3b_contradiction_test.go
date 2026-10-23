package learning

// Phase 3B contradiction development set and regression checks.
//
// The Phase 3A locked fixture (contradiction_v1.json) has already been observed,
// so any result on it here is a REGRESSION CHECK, not an independent holdout.
// New development cases below are generated separately and cover cross-kind
// conflicts, negation, temporal/version changes, project boundaries, and
// unrelated records.

import (
	"fmt"
	"os"
	"testing"

	"github.com/asymptote-labs/agent-beacon/pkg/asymptoteobserve"
)

type p3bCase struct {
	id       string
	category string
	kindA    string
	kindB    string
	projA    string
	projB    string
	verA     string
	verB     string
	a        string
	b        string
	conflict bool
}

func p3bCases() []p3bCase {
	c := []p3bCase{}
	add := func(cat, kindA, kindB, a, b string, conflict bool) {
		c = append(c, p3bCase{
			id: fmt.Sprintf("dev-%03d", len(c)+1), category: cat, kindA: kindA, kindB: kindB,
			projA: "alpha", projB: "alpha", a: a, b: b, conflict: conflict,
		})
	}

	// marker_conflict: opposite explicit polarity, same kind (baseline behaviour).
	add("marker_conflict", "convention", "convention",
		"Always refresh the token before retrying.", "Do not refresh the token before retrying.", true)
	add("marker_conflict", "workflow", "workflow",
		"Always checkpoint the WAL before copying the database.", "Never checkpoint the WAL before copying the database.", true)

	// antonym_conflict: opposite action verbs, no explicit polarity marker.
	add("antonym_conflict", "convention", "convention",
		"Always stop the writer when the lease expires.", "Continue the writer when the lease expires.", true)
	add("antonym_conflict", "workflow", "workflow",
		"Keep the previous reader active during the backfill.", "Remove the previous reader during the backfill.", true)
	add("antonym_conflict", "convention", "convention",
		"Enable the cache for the query path.", "Disable the cache for the query path.", true)
	add("antonym_conflict", "gotcha", "gotcha",
		"Allow retries after the rate limit.", "Deny retries after the rate limit.", true)
	add("antonym_conflict", "convention", "convention",
		"Include the digest in the cache key.", "Exclude the digest from the cache key.", true)

	// negation_conflict: one side negates the shared predicate.
	add("negation_conflict", "convention", "convention",
		"Preserve the last stable sort key in the cursor.", "Never preserve the last stable sort key in the cursor.", true)
	add("negation_conflict", "gotcha", "gotcha",
		"Record the model revision with each vector.", "Do not record the model revision with each vector.", true)
	add("negation_conflict", "workflow", "workflow",
		"Cache the bearer token in memory.", "Never cache the bearer token in memory.", true)

	// cross_kind_conflict: opposite polarity, different Kind (opt-in detection).
	add("cross_kind_conflict", "convention", "gotcha",
		"Always validate the schema before the migration.", "Never validate the schema before the migration.", true)
	add("cross_kind_conflict", "workflow", "correction",
		"Always checkpoint the WAL before copying.", "Do not checkpoint the WAL before copying.", true)
	add("cross_kind_conflict", "gotcha", "convention",
		"Always fence stale lease holders.", "Never fence stale lease holders.", true)

	// distinct_version: same kind/project, opposite polarity, explicit versions.
	{
		c = append(c,
			p3bCase{id: "dev-013", category: "distinct_version", kindA: "convention", kindB: "convention", projA: "alpha", projB: "alpha", verA: "v3", verB: "v4",
				a: "Always use offset pagination for the archive.", b: "Never use offset pagination for the archive.", conflict: false},
			p3bCase{id: "dev-014", category: "distinct_version", kindA: "workflow", kindB: "workflow", projA: "alpha", projB: "alpha", verA: "version 2.1", verB: "version 3.0",
				a: "Always accept the legacy bearer prefix.", b: "Do not accept the legacy bearer prefix.", conflict: false})
	}

	// cross_project: opposite polarity, different project.
	{
		c = append(c,
			p3bCase{id: "dev-015", category: "cross_project", kindA: "convention", kindB: "convention", projA: "alpha", projB: "beta",
				a: "Always keep the prior reader during backfill.", b: "Never keep the prior reader during backfill.", conflict: false},
			p3bCase{id: "dev-016", category: "cross_project", kindA: "gotcha", kindB: "gotcha", projA: "alpha", projB: "beta",
				a: "Always renew the lease before expiry.", b: "Do not renew the lease before expiry.", conflict: false})
	}

	// same_polarity: same polarity decoys (must not fire).
	add("same_polarity", "workflow", "workflow",
		"Always run the formatter before committing.", "Always run the linter before committing.", false)
	add("same_polarity", "convention", "convention",
		"Never ship a mismatched digest.", "Do not ship a mismatched digest.", false)

	// unrelated: opposite polarity but no shared subject.
	add("unrelated", "gotcha", "gotcha",
		"Always inspect the queue logs first.", "Never guess at the cache behaviour.", false)
	add("unrelated", "convention", "convention",
		"Always fence stale lease holders.", "Never ignore a signed manifest digest.", false)

	// insufficient_overlap: opposite polarity, no shared subject terms.
	add("insufficient_overlap", "correction", "correction",
		"Always pin the zebra widget.", "Do not cancel the yacht service.", false)
	add("insufficient_overlap", "correction", "correction",
		"Always rotate the orbiter.", "Do not purge the nebula.", false)

	return c
}

func p3bMemories(c p3bCase) (asymptoteobserve.LearningMemoryV1, asymptoteobserve.LearningMemoryV1) {
	mk := func(id, kind, project, version, body string) asymptoteobserve.LearningMemoryV1 {
		return asymptoteobserve.LearningMemoryV1{
			ID: id, CandidateID: "c-" + id, Kind: kind, Title: id, Body: body,
			Applicability: version, Project: asymptoteobserve.LearningProjectV1{ID: project},
			Evidence: []asymptoteobserve.LearningEvidenceV1{{TraceID: "trace-" + id}},
		}
	}
	return mk(c.id+"-a", c.kindA, c.projA, c.verA, c.a),
		mk(c.id+"-b", c.kindB, c.projB, c.verB, c.b)
}

func p3bScore(t *testing.T, cases []p3bCase, opts TrustOptions) map[string][4]int {
	t.Helper()
	perCat := map[string][4]int{} // tp, fp, fn, tn
	for _, c := range cases {
		ma, mb := p3bMemories(c)
		detected := len(detectContradictions([]asymptoteobserve.LearningMemoryV1{ma, mb}, opts)) > 0
		row := perCat[c.category]
		switch {
		case detected && c.conflict:
			row[0]++
		case detected && !c.conflict:
			row[1]++
		case !detected && c.conflict:
			row[2]++
		default:
			row[3]++
		}
		perCat[c.category] = row
	}
	return perCat
}

func p3bReport(t *testing.T, label string, perCat map[string][4]int) (tp, fp, fn, tn int) {
	t.Helper()
	for _, cat := range []string{"marker_conflict", "antonym_conflict", "negation_conflict", "cross_kind_conflict", "distinct_version", "cross_project", "same_polarity", "unrelated", "insufficient_overlap"} {
		row, ok := perCat[cat]
		if !ok {
			continue
		}
		tp += row[0]
		fp += row[1]
		fn += row[2]
		tn += row[3]
		fmt.Printf("P3B_DEV mode=%s category=%s tp=%d fp=%d fn=%d tn=%d\n", label, cat, row[0], row[1], row[2], row[3])
	}
	prec, rec := ratio(tp, tp+fp), ratio(tp, tp+fn)
	fmt.Printf("P3B_DEV mode=%s total tp=%d fp=%d fn=%d tn=%d precision=%.4f recall=%.4f\n", label, tp, fp, fn, tn, prec, rec)
	return
}

func TestPhase3BContradictionDev(t *testing.T) {
	cases := p3bCases()
	fmt.Printf("P3B_DEV cases=%d\n", len(cases))

	// Default mode: precision must be 1.0; all marker/antonym/negation conflicts
	// must be caught; cross-kind conflicts are expected to be missed by default.
	def := p3bScore(t, cases, TrustOptions{ContradictionTerms: 2})
	_, fp, _, _ := p3bReport(t, "default", def)
	if fp != 0 {
		t.Fatalf("default mode false positives: %d", fp)
	}
	for _, cat := range []string{"marker_conflict", "antonym_conflict", "negation_conflict"} {
		if row := def[cat]; row[2] != 0 {
			t.Fatalf("default mode missed %s conflicts: %d", cat, row[2])
		}
	}
	if row := def["cross_kind_conflict"]; row[0] != 0 {
		t.Fatalf("default mode detected cross-kind conflicts (%d); expected opt-in only", row[0])
	}

	// Opt-in cross-kind mode: precision must stay 1.0 and all conflicts detected.
	xk := p3bScore(t, cases, TrustOptions{ContradictionTerms: 2, AllowCrossKind: true})
	_, fpX, fnX, _ := p3bReport(t, "cross_kind", xk)
	if fpX != 0 {
		t.Fatalf("cross-kind mode false positives: %d", fpX)
	}
	if fnX != 0 {
		t.Fatalf("cross-kind mode missed conflicts: %d", fnX)
	}
}

// TestPhase3BLockedContradictionRegression re-measures the observed Phase 3A
// fixture. This is a regression check, not an independent evaluation.
func TestPhase3BLockedContradictionRegression(t *testing.T) {
	if os.Getenv("BEACON_PHASE3_CONTRADICTION") != "1" {
		t.Skip("set BEACON_PHASE3_CONTRADICTION=1")
	}
	fx := p3LoadContradictionFixture(t)
	score := func(label string, opts TrustOptions) {
		var tp, fp, fn, tn int
		byCat := map[string][4]int{}
		for _, p := range fx.Pairs {
			ma, mb := p3PairMemories(p)
			detected := len(detectContradictions([]asymptoteobserve.LearningMemoryV1{ma, mb}, opts)) > 0
			row := byCat[p.Category]
			switch {
			case detected && p.Conflict:
				row[0]++
			case detected && !p.Conflict:
				row[1]++
			case !detected && p.Conflict:
				row[2]++
			default:
				row[3]++
			}
			byCat[p.Category] = row
		}
		for _, cat := range []string{"same_kind", "cross_kind", "distinct_version", "cross_project", "same_polarity", "unrelated"} {
			row := byCat[cat]
			tp += row[0]
			fp += row[1]
			fn += row[2]
			tn += row[3]
			fmt.Printf("P3B_LOCKED mode=%s category=%s tp=%d fp=%d fn=%d tn=%d\n", label, cat, row[0], row[1], row[2], row[3])
		}
		fmt.Printf("P3B_LOCKED mode=%s total tp=%d fp=%d fn=%d tn=%d precision=%.4f recall=%.4f\n",
			label, tp, fp, fn, tn, ratio(tp, tp+fp), ratio(tp, tp+fn))
	}
	score("default", TrustOptions{ContradictionTerms: 2})
	score("cross_kind", TrustOptions{ContradictionTerms: 2, AllowCrossKind: true})
}
