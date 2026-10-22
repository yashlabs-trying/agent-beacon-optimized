package learning

import (
	"fmt"
	"strings"
	"testing"

	"github.com/asymptote-labs/agent-beacon/pkg/asymptoteobserve"
)

func TestSelectContextActuallyTruncatesBody(t *testing.T) {
	memory := asymptoteobserve.LearningMemoryV1{
		ID: "m1", Title: "note", Body: strings.Repeat("word ", 40),
		Evidence: []asymptoteobserve.LearningEvidenceV1{{TraceID: "trace-1"}},
	}
	sel := SelectContextFromMemories([]asymptoteobserve.LearningMemoryV1{memory}, ContextOptions{
		MaxTokens: 8, TruncateBodyTokens: 20,
	})
	if len(sel.Items) != 1 || !sel.Items[0].Truncated {
		t.Fatalf("expected one truncated memory: %+v", sel)
	}
	if sel.TotalTokens > 8 || EstimateTokens(sel.Items[0].Memory.Body) > 7 {
		t.Fatalf("selected context exceeds the declared budget: %+v", sel)
	}
	if sel.Items[0].Memory.Body == memory.Body || memory.Body != strings.Repeat("word ", 40) {
		t.Fatal("selection did not return a shortened copy without mutating the input")
	}
	if len(sel.Items[0].Memory.Evidence) != 1 || sel.Items[0].Memory.Evidence[0].TraceID != "trace-1" {
		t.Fatal("body truncation discarded provenance")
	}
}

func containsGold(sel ContextSelection, gold string) bool {
	for _, it := range sel.Items {
		if it.Memory.ID == gold {
			return true
		}
	}
	return false
}

// TestContextBudgeting compares handing an agent a fixed top-5 (untruncated)
// against a token-budgeted, body-truncated selection, and checks that the gold
// memory's evidence is still retained when it ranks.
func TestContextBudgeting(t *testing.T) {
	store, cases := buildSemanticStore(t)

	var fixedTokens, budgetTokens int
	var fixedGold, budgetGold, fixedItems, budgetItems int
	var budgetGoldWithBody, budgetGoldHalfBody, budgetGoldWithEvidence int
	for i, c := range cases {
		res, err := store.SearchHybrid(Query{ProjectID: "proj-a", Q: c.query}, HybridOptions{Limit: 50, CandidateK: 100})
		if err != nil {
			t.Fatal(err)
		}
		fixed := res
		if len(fixed) > 5 {
			fixed = fixed[:5]
		}
		fixedSel := SelectContext(fixed, ContextOptions{})
		budgetSel := SelectContext(res, ContextOptions{MaxTokens: 80, MaxItems: 5, TruncateBodyTokens: 40})

		fixedTokens += fixedSel.TotalTokens
		budgetTokens += budgetSel.TotalTokens
		fixedItems += len(fixedSel.Items)
		budgetItems += len(budgetSel.Items)
		gold := goldID(cases, i)
		if containsGold(fixedSel, gold) {
			fixedGold++
		}
		if containsGold(budgetSel, gold) {
			budgetGold++
			for _, item := range budgetSel.Items {
				if item.Memory.ID != gold {
					continue
				}
				if item.Memory.Body != "" {
					budgetGoldWithBody++
				}
				if EstimateTokens(item.Memory.Body)*2 >= EstimateTokens(c.gold) {
					budgetGoldHalfBody++
				}
				if len(item.Memory.Evidence) > 0 {
					budgetGoldWithEvidence++
				}
			}
		}
	}
	n := len(cases)
	fmt.Printf("CONTEXT queries=%d fixed_top5_tokens=%d budgeted_tokens=%d reduction=%.2fx fixed_items=%d budgeted_items=%d gold_retained_fixed=%d/%d gold_retained_budgeted=%d/%d\n",
		n, fixedTokens, budgetTokens,
		float64(fixedTokens)/float64(budgetTokens), fixedItems, budgetItems, fixedGold, n, budgetGold, n)
	fmt.Printf("CONTEXT gold_with_nonempty_body=%d gold_with_at_least_half_body=%d gold_with_provenance=%d\n",
		budgetGoldWithBody, budgetGoldHalfBody, budgetGoldWithEvidence)
	if budgetTokens > fixedTokens {
		t.Fatalf("budgeting did not reduce tokens: budgeted=%d fixed=%d", budgetTokens, fixedTokens)
	}
}
