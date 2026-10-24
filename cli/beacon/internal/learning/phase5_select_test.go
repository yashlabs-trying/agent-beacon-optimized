package learning

// Phase 5 selection dump. Reads the 300-task Phase 5 fixture and records, per task,
// the semantic retrieval (arm C candidates) and the intelligent-layer selection
// (arm D), writing JSON for the Python five-arm agent harness.
//
// Opt-in: BEACON_PHASE5_SELECT=1. Fixture via BEACON_PHASE5_FIXTURE, output via
// BEACON_PHASE5_OUT, K via BEACON_PHASE5_K (default 8). Neural via BEACON_EMBED_URL.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/asymptote-labs/agent-beacon/pkg/asymptoteobserve"
)

type p5Memory struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Body   string `json:"body"`
	Kind   string `json:"kind"`
	TS     string `json:"ts"`
	Source string `json:"source"`
}

type p5Task struct {
	ID             string      `json:"id"`
	Project        string      `json:"project"`
	RetrievalQuery string      `json:"retrieval_query"`
	Memories       []p5Memory  `json:"memories"`
}

type p5Fixture struct {
	Tasks []p5Task `json:"tasks"`
}

func p5ToMem(t p5Task, mm p5Memory) asymptoteobserve.LearningMemoryV1 {
	source := mm.Source
	if source == "" {
		source = "verified"
	}
	return asymptoteobserve.LearningMemoryV1{
		ID: mm.ID, CandidateID: "candidate-" + mm.ID, Kind: p4cKind(mm.Kind),
		Title: mm.Title, Body: mm.Body,
		Project:   asymptoteobserve.LearningProjectV1{ID: t.Project},
		Tags:      []string{"source:" + source},
		Evidence:  []asymptoteobserve.LearningEvidenceV1{{TraceID: "synthetic-phase5", EventIDs: []string{mm.ID}}},
		CreatedAt: mm.TS, UpdatedAt: mm.TS,
	}
}

func TestPhase5Select(t *testing.T) {
	if os.Getenv("BEACON_PHASE5_SELECT") != "1" {
		t.Skip("set BEACON_PHASE5_SELECT=1 to run the Phase 5 selection dump")
	}
	fixturePath := os.Getenv("BEACON_PHASE5_FIXTURE")
	outPath := os.Getenv("BEACON_PHASE5_OUT")
	if fixturePath == "" || outPath == "" {
		t.Fatal("BEACON_PHASE5_FIXTURE and BEACON_PHASE5_OUT are required")
	}
	k := 8
	if v := os.Getenv("BEACON_PHASE5_K"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			k = n
		}
	}
	raw, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var fx p5Fixture
	if err := json.Unmarshal(raw, &fx); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}

	var embed Embedder = NewHashedEmbedder()
	if url := os.Getenv("BEACON_EMBED_URL"); url != "" {
		if e, err := NewHTTPEmbedder(url); err == nil {
			embed = e
		}
	}

	intelligent := TrustContextOptions{
		EnableSupersession: true, EnableTrust: true, EnableConflict: true,
		EnableInjectionDefense: true, RequireSufficiency: true, WithholdIfInsufficient: true,
	}

	type out struct {
		Retrieved []string `json:"retrieved"`
		Selected  []string `json:"selected"`
	}
	res := map[string]out{}
	for _, tk := range fx.Tasks {
		store := OpenSemantic(filepath.Join(t.TempDir(), "p5.db"), embed)
		for _, mm := range tk.Memories {
			if err := store.PutMemory(p5ToMem(tk, mm)); err != nil {
				t.Fatalf("put: %v", err)
			}
		}
		hy, err := store.SearchHybrid(Query{ProjectID: tk.Project, Q: tk.RetrievalQuery},
			HybridOptions{Limit: k, CandidateK: 50, LexicalWeight: 0, SemanticWeight: 1})
		if err != nil {
			t.Fatalf("search %s: %v", tk.ID, err)
		}
		var retrieved []asymptoteobserve.LearningMemoryV1
		var rids []string
		for _, r := range hy {
			retrieved = append(retrieved, r.Memory)
			rids = append(rids, r.Memory.ID)
		}
		sel := SelectTrustContext(retrieved, tk.RetrievalQuery, intelligent)
		var sids []string
		for _, mm := range sel.Memories {
			sids = append(sids, mm.ID)
		}
		res[tk.ID] = out{Retrieved: rids, Selected: sids}
		_ = store.Close()
	}
	blob, _ := json.MarshalIndent(map[string]any{"k": k, "tasks": res}, "", "  ")
	if err := os.WriteFile(outPath, append(blob, '\n'), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	fmt.Printf("P5_SELECT tasks=%d\n", len(fx.Tasks))
}
