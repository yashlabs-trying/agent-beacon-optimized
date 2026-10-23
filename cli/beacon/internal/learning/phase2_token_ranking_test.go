package learning

import (
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/asymptote-labs/agent-beacon/pkg/asymptoteobserve"
)

// phase2_token_ranking_test.go evaluates (a) context-selection strategies under a
// token budget and (b) hybrid ranking alternatives, on a larger deterministic
// labeled dataset split into tuning and held-out halves. Tuning picks a
// configuration; the held-out half is the number that matters. Nothing here
// changes production defaults: the weighting knobs exist, but a strategy is only
// reported, not enabled.

var dNouns = []string{
	"cache", "token", "worker", "index", "queue", "shard", "replica", "session",
	"payload", "socket", "buffer", "cursor", "timer", "mutex", "lease", "bucket",
	"stream", "digest", "ledger", "manifest", "broker", "router", "schema", "journal",
}
var dVerbs = []string{
	"stampede", "expiry", "drain", "bloat", "backlog", "skew", "drift", "leak",
	"corruption", "stall", "thrash", "race", "deadlock", "flap", "poison", "replay",
	"reorder", "starve", "jitter", "fragmentation", "contention", "throttle", "spill", "starvation",
}

type dQuery struct {
	topic    int
	category string
	text     string
	gold     string
	relevant []string
}

func buildRankingDataset(t *testing.T) (*Store, []dQuery) {
	t.Helper()
	store := OpenSemantic(filepath.Join(t.TempDir(), "memory.db"), NewHashedEmbedder())
	project := asymptoteobserve.LearningProjectV1{ID: "proj-d", Path: "/repos/d"}
	put := func(id, body, trace string, verified bool) {
		m := asymptoteobserve.LearningMemoryV1{
			ID: id, CandidateID: "c-" + id, Kind: asymptoteobserve.LearningMemoryKindDebuggingPattern,
			Title: "lesson", Body: body, Project: project,
		}
		if verified {
			m.Evidence = []asymptoteobserve.LearningEvidenceV1{{TraceID: trace}}
		}
		if err := store.PutMemory(m); err != nil {
			t.Fatal(err)
		}
	}

	n := len(dNouns)
	for i := 0; i < n; i++ {
		noun, verb := dNouns[i], dVerbs[i]
		code := fmt.Sprintf("ZQ%d", 100+i)
		ident := fmt.Sprintf("ERR_%s_%d", strings.ToUpper(noun[:3]), 1000+i)
		trace := fmt.Sprintf("tr-topic-%d", i)
		goldBody := fmt.Sprintf("The %s %s condition requires clearing %s and handling %s before retrying.", noun, verb, code, ident)
		put(fmt.Sprintf("gold-%02d", i), goldBody, trace, true)
		// duplicate: same evidence trace and near-identical text as the gold.
		put(fmt.Sprintf("dup-%02d", i), goldBody+" (restated)", trace, true)
		// related / related2: same subject, weaker, distinct evidence.
		put(fmt.Sprintf("related-%02d", i),
			fmt.Sprintf("A %s %s symptom is usually transient and clears on its own.", noun, verb), fmt.Sprintf("tr-rel-%d", i), true)
		put(fmt.Sprintf("related2-%02d", i),
			fmt.Sprintf("Handle the %s %s by clearing %s first.", noun, verb, code), fmt.Sprintf("tr-rel2-%d", i), true)
	}
	// Generic distractors sharing the common vocabulary; half are unverified.
	for i := 0; i < 40; i++ {
		put(fmt.Sprintf("distractor-%02d", i),
			fmt.Sprintf("The service should always log the request %s and retry on a transient failure %d.", dNouns[i%n], i),
			fmt.Sprintf("tr-dist-%d", i), i%2 == 0)
	}

	var queries []dQuery
	for i := 0; i < n; i++ {
		noun, verb := dNouns[i], dVerbs[i]
		code := fmt.Sprintf("ZQ%d", 100+i)
		ident := fmt.Sprintf("ERR_%s_%d", strings.ToUpper(noun[:3]), 1000+i)
		gold := fmt.Sprintf("gold-%02d", i)
		// dup carries the same evidence and content as gold, so it satisfies the
		// same information need; a strategy that drops gold in favour of dup
		// loses nothing.
		rel := []string{gold, fmt.Sprintf("dup-%02d", i), fmt.Sprintf("related-%02d", i), fmt.Sprintf("related2-%02d", i)}
		queries = append(queries,
			dQuery{i, "exact", code, gold, rel},
			dQuery{i, "identifier", ident, gold, rel},
			dQuery{i, "overlap", noun + " " + verb, gold, rel},
			// Morphological variant: the hashed embedder can bridge these.
			dQuery{i, "morph", noun + "s " + verb + "ing", gold, rel},
		)
	}
	return store, queries
}

type rankMetrics struct{ n, r1, r5, mrr, ndcg float64 }

func (m rankMetrics) add(ids []string, gold string) rankMetrics {
	m.n++
	r1, r5, rr, ndcg := evaluateRanking(ids, gold)
	m.r1 += r1
	m.r5 += r5
	m.mrr += rr
	m.ndcg += ndcg
	return m
}

func (m rankMetrics) report(name string) string {
	return fmt.Sprintf("%s n=%.0f R@1=%.3f R@5=%.3f MRR=%.3f NDCG@5=%.3f",
		name, m.n, m.r1/m.n, m.r5/m.n, m.mrr/m.n, m.ndcg/m.n)
}

func splitQueries(qs []dQuery) (tune, held []dQuery) {
	for _, q := range qs {
		if q.topic%2 == 0 {
			tune = append(tune, q)
		} else {
			held = append(held, q)
		}
	}
	return
}

func TestPhase2RankingAlternatives(t *testing.T) {
	store, queries := buildRankingDataset(t)
	tune, held := splitQueries(queries)
	db, err := store.db()
	if err != nil {
		t.Fatal(err)
	}

	methods := []struct {
		name string
		run  func(q dQuery) []string
	}{
		{"lexical", func(q dQuery) []string {
			got, err := store.ListMemories(Query{ProjectID: "proj-d", Q: q.text, Limit: 5})
			if err != nil {
				t.Fatal(err)
			}
			return ids(got)
		}},
		{"semantic", func(q dQuery) []string {
			hits, err := store.semanticSearchExact(db, Query{ProjectID: "proj-d"}, store.embedder.Embed(q.text), 5)
			if err != nil {
				t.Fatal(err)
			}
			out := make([]string, len(hits))
			for i, h := range hits {
				out[i] = h.memory.ID
			}
			return out
		}},
		{"hybrid_rrf", func(q dQuery) []string {
			got, err := store.SearchHybrid(Query{ProjectID: "proj-d", Q: q.text}, HybridOptions{Limit: 5, CandidateK: 50})
			if err != nil {
				t.Fatal(err)
			}
			return hybridIDs(got)
		}},
		{"hybrid_semantic_heavy", func(q dQuery) []string {
			got, err := store.SearchHybrid(Query{ProjectID: "proj-d", Q: q.text}, HybridOptions{Limit: 5, CandidateK: 50, SemanticWeight: 3, LexicalWeight: 1})
			if err != nil {
				t.Fatal(err)
			}
			return hybridIDs(got)
		}},
		{"hybrid_lexical_heavy", func(q dQuery) []string {
			got, err := store.SearchHybrid(Query{ProjectID: "proj-d", Q: q.text}, HybridOptions{Limit: 5, CandidateK: 50, LexicalWeight: 3, SemanticWeight: 1})
			if err != nil {
				t.Fatal(err)
			}
			return hybridIDs(got)
		}},
		{"hybrid_identifier_boost", func(q dQuery) []string {
			lw, sw := 1.0, 1.0
			if queryLooksLikeIdentifier(q.text) {
				lw = 4
			}
			got, err := store.SearchHybrid(Query{ProjectID: "proj-d", Q: q.text}, HybridOptions{Limit: 5, CandidateK: 50, LexicalWeight: lw, SemanticWeight: sw})
			if err != nil {
				t.Fatal(err)
			}
			return hybridIDs(got)
		}},
	}

	for _, m := range methods {
		tm, hm := rankMetrics{}, rankMetrics{}
		for _, q := range tune {
			tm = tm.add(m.run(q), q.gold)
		}
		for _, q := range held {
			hm = hm.add(m.run(q), q.gold)
		}
		t.Logf("RANK %s | %s | %s", m.name, tm.report("tune"), hm.report("held-out"))
	}
}

func hybridIDs(results []HybridResult) []string {
	out := make([]string, len(results))
	for i, r := range results {
		out[i] = r.Memory.ID
	}
	return out
}

// queryLooksLikeIdentifier reports whether a query is dominated by an
// identifier-like token (an underscore, or an uppercase-with-digit run). Such
// queries are exact-identifier lookups where lexical match should dominate.
func queryLooksLikeIdentifier(q string) bool {
	for _, tok := range strings.Fields(q) {
		if strings.Contains(tok, "_") {
			return true
		}
		hasDigit, hasUpper := false, false
		for _, r := range tok {
			if r >= '0' && r <= '9' {
				hasDigit = true
			}
			if r >= 'A' && r <= 'Z' {
				hasUpper = true
			}
		}
		if hasDigit && hasUpper {
			return true
		}
	}
	return false
}

// ---- Token selection strategies ----

// tokenStrategies are pure functions over a ranked result list, returning the
// items they would hand to an agent. They are experiments: only a strategy that
// wins on held-out data would be promoted to production.
func tokenStrategies(store *Store) []struct {
	name string
	run  func(results []HybridResult) []HybridResult
} {
	emb := store.embedder
	body := func(r HybridResult) string { return r.Memory.Title + "\n" + r.Memory.Body }
	return []struct {
		name string
		run  func(results []HybridResult) []HybridResult
	}{
		{"fixed_top5", func(results []HybridResult) []HybridResult {
			return results[:min(5, len(results))]
		}},
		{"token_budget", func(results []HybridResult) []HybridResult {
			c := SelectContext(results, ContextOptions{MaxTokens: 80, MaxItems: 5, TruncateBodyTokens: 40})
			out := make([]HybridResult, len(c.Items))
			for i, it := range c.Items {
				out[i] = HybridResult{Memory: it.Memory}
			}
			return out
		}},
		{"evidence_dedup", func(results []HybridResult) []HybridResult {
			var sel []HybridResult
			seen := map[string]bool{}
			for _, r := range results {
				key := evidenceKey(r.Memory)
				if key != "" && seen[key] {
					continue
				}
				seen[key] = true
				sel = append(sel, r)
				if len(sel) == 5 {
					break
				}
			}
			return sel
		}},
		{"trust_aware", func(results []HybridResult) []HybridResult {
			trust := map[string]TrustStatus{}
			for _, tm := range AnnotateTrust(memoriesOf(results), TrustOptions{MaxAgeDays: 180}) {
				trust[tm.Memory.ID] = tm.Status
			}
			var verified, rest []HybridResult
			for _, r := range results {
				if trust[r.Memory.ID] == TrustVerified {
					verified = append(verified, r)
				} else {
					rest = append(rest, r)
				}
			}
			sel := append(verified, rest...)
			return sel[:min(5, len(sel))]
		}},
		{"diversity_mmr", func(results []HybridResult) []HybridResult {
			var sel []HybridResult
			pool := append([]HybridResult(nil), results...)
			for len(sel) < 5 && len(pool) > 0 {
				bestIdx, bestScore := -1, math.Inf(-1)
				for i, r := range pool {
					maxSim := 0.0
					for _, s := range sel {
						sim := float64(cosine(emb.Embed(body(r)), emb.Embed(body(s))))
						if sim > maxSim {
							maxSim = sim
						}
					}
					score := (1.0 / float64(i+1)) - 0.5*maxSim
					if score > bestScore {
						bestScore, bestIdx = score, i
					}
				}
				sel = append(sel, pool[bestIdx])
				pool = append(pool[:bestIdx], pool[bestIdx+1:]...)
			}
			return sel
		}},
	}
}

// evidenceKey identifies the provenance of a memory for deduplication.
func evidenceKey(m asymptoteobserve.LearningMemoryV1) string {
	var parts []string
	for _, e := range m.Evidence {
		parts = append(parts, e.TraceID)
	}
	return strings.Join(parts, "|")
}

func tokensOf(results []HybridResult) int {
	total := 0
	for _, r := range results {
		total += EstimateTokens(r.Memory.Title) + EstimateTokens(r.Memory.Body)
	}
	return total
}

func memoriesOf(results []HybridResult) []asymptoteobserve.LearningMemoryV1 {
	out := make([]asymptoteobserve.LearningMemoryV1, len(results))
	for i, r := range results {
		out[i] = r.Memory
	}
	return out
}

func redundancyOf(sel []HybridResult, store *Store) float64 {
	if len(sel) < 2 {
		return 0
	}
	emb := store.embedder
	var sum float64
	var pairs int
	for i := 0; i < len(sel); i++ {
		for j := i + 1; j < len(sel); j++ {
			bi := emb.Embed(sel[i].Memory.Title + "\n" + sel[i].Memory.Body)
			bj := emb.Embed(sel[j].Memory.Title + "\n" + sel[j].Memory.Body)
			sum += float64(cosine(bi, bj))
			pairs++
		}
	}
	return sum / float64(pairs)
}

func TestPhase2TokenSelectionStrategies(t *testing.T) {
	store, queries := buildRankingDataset(t)
	_, held := splitQueries(queries)
	strategies := tokenStrategies(store)

	type agg struct {
		tokens, relevant, relTotal, items int
		redund, latencyMs                 float64
		cases                             int
	}
	results := make([]agg, len(strategies))

	for _, q := range held {
		ranked, err := store.SearchHybrid(Query{ProjectID: "proj-d", Q: q.text}, HybridOptions{Limit: 50, CandidateK: 100})
		if err != nil {
			t.Fatal(err)
		}
		// Relevant findings form three distinct groups: the primary finding
		// (carried by gold and its duplicate), and two supporting findings.
		groups := [][]string{
			{fmt.Sprintf("gold-%02d", q.topic), fmt.Sprintf("dup-%02d", q.topic)},
			{fmt.Sprintf("related-%02d", q.topic)},
			{fmt.Sprintf("related2-%02d", q.topic)},
		}
		for si, s := range strategies {
			start := time.Now()
			sel := s.run(ranked)
			results[si].latencyMs += float64(time.Since(start).Microseconds()) / 1000
			results[si].tokens += tokensOf(sel)
			results[si].items += len(sel)
			results[si].redund += redundancyOf(sel, store)
			results[si].cases++
			results[si].relTotal += len(groups)
			got := map[string]bool{}
			for _, r := range sel {
				got[r.Memory.ID] = true
			}
			for _, g := range groups {
				for _, id := range g {
					if got[id] {
						results[si].relevant++
						break
					}
				}
			}
		}
	}

	for si, s := range strategies {
		a := results[si]
		t.Logf("TOKEN strategy=%-16s cases=%d tokens=%d tokens/case=%.1f relevant_retained=%d/%d redundancy=%.3f latency_us/case=%.1f",
			s.name, a.cases, a.tokens, float64(a.tokens)/float64(a.cases),
			a.relevant, a.relTotal, a.redund/float64(a.cases), a.latencyMs*1000/float64(a.cases))
	}
}
