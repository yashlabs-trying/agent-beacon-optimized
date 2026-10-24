package learning

// Phase 4B in-process retrieval harness. The CLI exposes no semantic flags, so
// hashed/hybrid/neural retrieval must be exercised through the Go API. This
// opt-in test seeds each Phase 4B case's memories into a fresh store and records
// the top-K ranked IDs for every method, writing JSON for the Python agent pilot.
//
// Opt-in: set BEACON_PHASE4B_RETRIEVAL=1. Fixture path via BEACON_PHASE4B_FIXTURE,
// output path via BEACON_PHASE4B_OUT, K via BEACON_PHASE4B_K (default 5).
// Neural runs only when BEACON_EMBED_URL is set.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/asymptote-labs/agent-beacon/pkg/asymptoteobserve"
)

type p4bMemory struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Body  string `json:"body"`
	Kind  string `json:"kind"`
	TS    string `json:"ts"`
}

type p4bCase struct {
	ID       string      `json:"id"`
	Project  string      `json:"project"`
	Query    string      `json:"query"`
	Memories []p4bMemory `json:"memories"`
}

type p4bFixture struct {
	Cases []p4bCase `json:"cases"`
}

func p4bKind(k string) string {
	switch k {
	case "gotcha":
		return asymptoteobserve.LearningMemoryKindGotcha
	case "workflow":
		return asymptoteobserve.LearningMemoryKindWorkflow
	default:
		return asymptoteobserve.LearningMemoryKindConvention
	}
}

func p4bSeed(store *Store, c p4bCase) {
	for _, m := range c.Memories {
		mem := asymptoteobserve.LearningMemoryV1{
			ID: m.ID, CandidateID: "candidate-" + m.ID, Kind: p4bKind(m.Kind),
			Title: m.Title, Body: m.Body,
			Project:  asymptoteobserve.LearningProjectV1{ID: c.Project},
			Evidence: []asymptoteobserve.LearningEvidenceV1{{TraceID: "synthetic-phase4b", EventIDs: []string{m.ID}}},
		}
		if err := store.PutMemory(mem); err != nil {
			panic(err)
		}
	}
}

func p4bIDs(res []HybridResult) []string {
	out := make([]string, 0, len(res))
	for _, r := range res {
		out = append(out, r.Memory.ID)
	}
	return out
}

func TestPhase4BRetrieval(t *testing.T) {
	if os.Getenv("BEACON_PHASE4B_RETRIEVAL") != "1" {
		t.Skip("set BEACON_PHASE4B_RETRIEVAL=1 to run the Phase 4B retrieval dump")
	}
	fixturePath := os.Getenv("BEACON_PHASE4B_FIXTURE")
	outPath := os.Getenv("BEACON_PHASE4B_OUT")
	if fixturePath == "" || outPath == "" {
		t.Fatal("BEACON_PHASE4B_FIXTURE and BEACON_PHASE4B_OUT are required")
	}
	k := 5
	if v := os.Getenv("BEACON_PHASE4B_K"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			k = n
		}
	}
	raw, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var fx p4bFixture
	if err := json.Unmarshal(raw, &fx); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}

	var embed Embedder
	if url := os.Getenv("BEACON_EMBED_URL"); url != "" {
		if e, err := NewHTTPEmbedder(url); err == nil {
			embed = e
		} else {
			fmt.Printf("P4B_NEURAL status=BLOCKED err=%q\n", err.Error())
		}
	}

	type result struct {
		Lexical []string `json:"lexical"`
		Hashed  []string `json:"hashed"`
		Hybrid  []string `json:"hybrid"`
		Neural  []string `json:"neural"`
	}
	out := map[string]result{}
	for _, c := range fx.Cases {
		var r result
		lex := Open(filepath.Join(t.TempDir(), "lex.db"))
		p4bSeed(lex, c)
		got, err := lex.ListMemories(Query{ProjectID: c.Project, Q: c.Query, Limit: k})
		if err != nil {
			t.Fatalf("lexical: %v", err)
		}
		for _, m := range got {
			r.Lexical = append(r.Lexical, m.ID)
		}
		_ = lex.Close()

		hashed := OpenSemantic(filepath.Join(t.TempDir(), "hashed.db"), NewHashedEmbedder())
		p4bSeed(hashed, c)
		res, err := hashed.SearchHybrid(Query{ProjectID: c.Project, Q: c.Query},
			HybridOptions{Limit: k, CandidateK: 50, LexicalWeight: 0, SemanticWeight: 1})
		if err != nil {
			t.Fatalf("hashed: %v", err)
		}
		r.Hashed = p4bIDs(res)
		res, err = hashed.SearchHybrid(Query{ProjectID: c.Project, Q: c.Query},
			HybridOptions{Limit: k, CandidateK: 50})
		if err != nil {
			t.Fatalf("hybrid: %v", err)
		}
		r.Hybrid = p4bIDs(res)
		_ = hashed.Close()

		if embed != nil {
			neural := OpenSemantic(filepath.Join(t.TempDir(), "neural.db"), embed)
			p4bSeed(neural, c)
			res, err := neural.SearchHybrid(Query{ProjectID: c.Project, Q: c.Query},
				HybridOptions{Limit: k, CandidateK: 50, LexicalWeight: 0, SemanticWeight: 1})
			if err != nil {
				t.Fatalf("neural: %v", err)
			}
			r.Neural = p4bIDs(res)
			_ = neural.Close()
		}
		out[c.ID] = r
		fmt.Printf("P4B_RETRIEVAL case=%s lexical=%v hashed=%v hybrid=%v neural=%v\n",
			c.ID, r.Lexical, r.Hashed, r.Hybrid, r.Neural)
	}
	blob, _ := json.MarshalIndent(map[string]any{"k": k, "methods": out}, "", "  ")
	if err := os.WriteFile(outPath, append(blob, '\n'), 0o644); err != nil {
		t.Fatalf("write out: %v", err)
	}
}
