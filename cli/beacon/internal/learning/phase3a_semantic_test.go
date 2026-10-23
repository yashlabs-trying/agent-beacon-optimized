package learning

// Phase 3A Workstream A (V6-only arms): hashed-semantic, hybrid RRF, and
// optional real-neural retrieval over the same locked 48-query fixture. This
// file needs the V6 API (OpenSemantic/SearchHybrid) and therefore is only
// compiled on the Phase 3A worktree, not on the pre-V6 baseline.

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/asymptote-labs/agent-beacon/pkg/asymptoteobserve"
)

func p3SeedSemantic(t *testing.T, fx p3RetrievalFixture, e Embedder) *Store {
	t.Helper()
	store := OpenSemantic(filepath.Join(t.TempDir(), "memory.db"), e)
	for _, topic := range fx.Topics {
		mems := []asymptoteobserve.LearningMemoryV1{
			p3Memory(topic.ID+"-primary", fx.ProjectID, "convention", topic.ID+" primary", topic.Primary, topic.ID+"-primary"),
			p3Memory(topic.ID+"-duplicate", fx.ProjectID, "convention", topic.ID+" duplicate", topic.Duplicate, topic.ID+"-primary"),
			p3Memory(topic.ID+"-support", fx.ProjectID, "gotcha", topic.ID+" support", topic.Support, topic.ID+"-support"),
			p3Memory(topic.ID+"-decoy", fx.ProjectID, "workflow", topic.ID+" decoy", topic.Decoy, topic.ID+"-decoy"),
		}
		for _, m := range mems {
			if err := store.PutMemory(m); err != nil {
				t.Fatalf("put %s: %v", m.ID, err)
			}
		}
	}
	return store
}

func p3HybridIDs(res []HybridResult) []string {
	ids := make([]string, 0, len(res))
	for _, r := range res {
		ids = append(ids, r.Memory.ID)
	}
	return ids
}

func TestPhase3ARetrievalSemantic(t *testing.T) {
	if os.Getenv("BEACON_PHASE3_RETRIEVAL") != "1" {
		t.Skip("set BEACON_PHASE3_RETRIEVAL=1 to run the locked retrieval evaluation")
	}
	fx := p3LoadRetrievalFixture(t)
	cases := p3QueryCases(fx)

	hashed := p3SeedSemantic(t, fx, NewHashedEmbedder())
	p3RunMethod("semantic_hashed", cases, func(c p3QueryCase) []string {
		res, err := hashed.SearchHybrid(Query{ProjectID: fx.ProjectID, Q: c.Text},
			HybridOptions{Limit: 5, CandidateK: 50, LexicalWeight: 0, SemanticWeight: 1})
		if err != nil {
			t.Fatalf("hashed semantic: %v", err)
		}
		return p3HybridIDs(res)
	})

	p3RunMethod("hybrid_rrf", cases, func(c p3QueryCase) []string {
		res, err := hashed.SearchHybrid(Query{ProjectID: fx.ProjectID, Q: c.Text},
			HybridOptions{Limit: 5, CandidateK: 50})
		if err != nil {
			t.Fatalf("hybrid rrf: %v", err)
		}
		return p3HybridIDs(res)
	})

	p3RunMethod("hybrid_ann", cases, func(c p3QueryCase) []string {
		res, err := hashed.SearchHybrid(Query{ProjectID: fx.ProjectID, Q: c.Text},
			HybridOptions{Limit: 5, CandidateK: 50, UseANN: true})
		if err != nil {
			t.Fatalf("hybrid ann: %v", err)
		}
		return p3HybridIDs(res)
	})

	// Real neural arm: only when a local BGE sidecar is reachable. Otherwise
	// mark BLOCKED rather than fabricating results.
	url := os.Getenv("BEACON_EMBED_URL")
	if url == "" {
		fmt.Printf("P3A_RETRIEVAL method=neural status=BLOCKED reason=no_BEACON_EMBED_URL sidecar_unavailable\n")
		return
	}
	emb, err := NewHTTPEmbedder(url)
	if err != nil {
		fmt.Printf("P3A_RETRIEVAL method=neural status=BLOCKED reason=sidecar_unreachable err=%q\n", err.Error())
		return
	}
	fmt.Printf("P3A_RETRIEVAL method=neural status=RUN model=%s dim=%d\n", emb.Model(), emb.Dim())
	neural := p3SeedSemantic(t, fx, emb)
	p3RunMethod("neural_bge", cases, func(c p3QueryCase) []string {
		res, err := neural.SearchHybrid(Query{ProjectID: fx.ProjectID, Q: c.Text},
			HybridOptions{Limit: 5, CandidateK: 50, LexicalWeight: 0, SemanticWeight: 1})
		if err != nil {
			t.Fatalf("neural semantic: %v", err)
		}
		return p3HybridIDs(res)
	})
}
