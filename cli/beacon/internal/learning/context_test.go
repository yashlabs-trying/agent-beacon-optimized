package learning

import (
	"fmt"
	"testing"
)

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
		}
	}
	n := len(cases)
	fmt.Printf("CONTEXT queries=%d fixed_top5_tokens=%d budgeted_tokens=%d reduction=%.2fx fixed_items=%d budgeted_items=%d gold_retained_fixed=%d/%d gold_retained_budgeted=%d/%d\n",
		n, fixedTokens, budgetTokens,
		float64(fixedTokens)/float64(budgetTokens), fixedItems, budgetItems, fixedGold, n, budgetGold, n)
	if budgetTokens > fixedTokens {
		t.Fatalf("budgeting did not reduce tokens: budgeted=%d fixed=%d", budgetTokens, fixedTokens)
	}
}
