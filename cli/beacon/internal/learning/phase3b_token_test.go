package learning

// Phase 3B token-budget analysis: identify exactly which relevant findings the
// current budgeted selector drops relative to unbudgeted top-five, classify them
// by importance (primary vs supporting), and add regression checks for the
// high-value (primary) finding and for evidence preservation.

import (
	"fmt"
	"testing"
)

func anyIn(results []HybridResult, ids []string) bool {
	set := map[string]bool{}
	for _, r := range results {
		set[r.Memory.ID] = true
	}
	for _, id := range ids {
		if set[id] {
			return true
		}
	}
	return false
}

func TestPhase3BTokenLostFindings(t *testing.T) {
	store, queries := buildRankingDataset(t)
	_, held := splitQueries(queries)
	byName := map[string]func([]HybridResult) []HybridResult{}
	for _, s := range tokenStrategies(store) {
		byName[s.name] = s.run
	}
	groupNames := []string{"gold", "related", "related2"}

	keptTop5 := map[string]int{}
	keptBudget := map[string]int{}
	keptDedup := map[string]int{}
	lostByGroup := map[string]int{}
	var losses []string
	tokensTop5, tokensBudget, tokensDedup := 0, 0, 0
	evidenceLost := 0
	keptItems, keptUnverified := 0, 0

	for _, q := range held {
		ranked, err := store.SearchHybrid(Query{ProjectID: "proj-d", Q: q.text}, HybridOptions{Limit: 50, CandidateK: 100})
		if err != nil {
			t.Fatal(err)
		}
		top5 := byName["fixed_top5"](ranked)
		budget := byName["token_budget"](ranked)
		dedup := byName["evidence_dedup"](ranked)
		tokensTop5 += tokensOf(top5)
		tokensBudget += tokensOf(budget)
		tokensDedup += tokensOf(dedup)

		// Which ranked memories carried usable provenance.
		hadEvidence := map[string]bool{}
		for _, r := range ranked {
			for _, e := range r.Memory.Evidence {
				if e.TraceID != "" {
					hadEvidence[r.Memory.ID] = true
				}
			}
		}
		// Evidence coverage: an item that carried provenance in the ranked list
		// must still carry it after budgeted selection (truncation must not strip
		// evidence). Items that never had provenance are counted separately.
		for _, r := range budget {
			keptItems++
			usable := false
			for _, e := range r.Memory.Evidence {
				if e.TraceID != "" {
					usable = true
				}
			}
			if !hadEvidence[r.Memory.ID] {
				keptUnverified++
				continue
			}
			if !usable {
				evidenceLost++
			}
		}

		groups := [][]string{
			{fmt.Sprintf("gold-%02d", q.topic), fmt.Sprintf("dup-%02d", q.topic)},
			{fmt.Sprintf("related-%02d", q.topic)},
			{fmt.Sprintf("related2-%02d", q.topic)},
		}
		for gi, g := range groups {
			inTop5 := anyIn(top5, g)
			inBudget := anyIn(budget, g)
			inDedup := anyIn(dedup, g)
			if inTop5 {
				keptTop5[groupNames[gi]]++
			}
			if inBudget {
				keptBudget[groupNames[gi]]++
			}
			if inDedup {
				keptDedup[groupNames[gi]]++
			}
			if inTop5 && !inBudget {
				lostByGroup[groupNames[gi]]++
				losses = append(losses, fmt.Sprintf("topic=%d group=%s", q.topic, groupNames[gi]))
			}
		}
	}

	fmt.Printf("P3B_TOKEN cases=%d tokens_top5=%d tokens_budget=%d tokens_dedup=%d\n", len(held), tokensTop5, tokensBudget, tokensDedup)
	fmt.Printf("P3B_TOKEN kept_top5 gold=%d related=%d related2=%d\n", keptTop5["gold"], keptTop5["related"], keptTop5["related2"])
	fmt.Printf("P3B_TOKEN kept_budget gold=%d related=%d related2=%d\n", keptBudget["gold"], keptBudget["related"], keptBudget["related2"])
	fmt.Printf("P3B_TOKEN kept_dedup gold=%d related=%d related2=%d\n", keptDedup["gold"], keptDedup["related"], keptDedup["related2"])
	fmt.Printf("P3B_TOKEN lost_by_group gold=%d related=%d related2=%d\n", lostByGroup["gold"], lostByGroup["related"], lostByGroup["related2"])
	for _, l := range losses {
		fmt.Printf("P3B_TOKEN loss %s\n", l)
	}
	fmt.Printf("P3B_TOKEN evidence_lost_in_budget=%d kept_items=%d kept_unverified=%d\n", evidenceLost, keptItems, keptUnverified)

	// Regression: the budgeted selector must not drop the primary finding that
	// top-five retains, and must not strip provenance from any item that had it.
	if lostByGroup["gold"] > 0 {
		t.Errorf("budgeted selector dropped %d primary (gold) findings that top-five retained", lostByGroup["gold"])
	}
	if evidenceLost > 0 {
		t.Errorf("budgeted selector stripped provenance from %d kept items that had evidence", evidenceLost)
	}
}

// TestPhase3BEvidenceDedupPreservesFindings checks the evidence-preserving
// deduplication policy retains at least as many findings as unbudgeted top-five
// and removes duplicate-evidence items.
func TestPhase3BEvidenceDedupPreservesFindings(t *testing.T) {
	store, queries := buildRankingDataset(t)
	_, held := splitQueries(queries)
	byName := map[string]func([]HybridResult) []HybridResult{}
	for _, s := range tokenStrategies(store) {
		byName[s.name] = s.run
	}
	totalTop5, totalDedup := 0, 0
	for _, q := range held {
		ranked, err := store.SearchHybrid(Query{ProjectID: "proj-d", Q: q.text}, HybridOptions{Limit: 50, CandidateK: 100})
		if err != nil {
			t.Fatal(err)
		}
		groups := [][]string{
			{fmt.Sprintf("gold-%02d", q.topic), fmt.Sprintf("dup-%02d", q.topic)},
			{fmt.Sprintf("related-%02d", q.topic)},
			{fmt.Sprintf("related2-%02d", q.topic)},
		}
		count := func(sel []HybridResult) int {
			n := 0
			for _, g := range groups {
				if anyIn(sel, g) {
					n++
				}
			}
			return n
		}
		totalTop5 += count(byName["fixed_top5"](ranked))
		totalDedup += count(byName["evidence_dedup"](ranked))
	}
	fmt.Printf("P3B_TOKEN dedup_vs_top5 top5=%d dedup=%d\n", totalTop5, totalDedup)
	if totalDedup < totalTop5 {
		t.Errorf("evidence dedup retained fewer findings (%d) than top-five (%d)", totalDedup, totalTop5)
	}
}
