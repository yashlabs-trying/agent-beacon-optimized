package learning

// Phase 4C selection-dump harness. Retrieves top-K semantic evidence per case and
// records the output of SelectTrustContext under each ablation configuration, so
// the Python pilot can build agent prompts for the semantic and intelligent arms
// and for each isolated mechanism.
//
// Opt-in: BEACON_PHASE4C_SELECT=1. Fixture via BEACON_PHASE4C_FIXTURE, output via
// BEACON_PHASE4C_OUT, K via BEACON_PHASE4C_K (default 8). Neural uses
// BEACON_EMBED_URL when set; hashed is always available.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/asymptote-labs/agent-beacon/pkg/asymptoteobserve"
)

type p4cMemory struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Body   string `json:"body"`
	Kind   string `json:"kind"`
	TS     string `json:"ts"`
	Source string `json:"source"`
}

type p4cCase struct {
	ID       string      `json:"id"`
	Project  string      `json:"project"`
	Query    string      `json:"query"`
	Memories []p4cMemory `json:"memories"`
}

type p4cFixture struct {
	Cases []p4cCase `json:"cases"`
}

func p4cKind(k string) string {
	switch k {
	case "gotcha":
		return asymptoteobserve.LearningMemoryKindGotcha
	case "workflow":
		return asymptoteobserve.LearningMemoryKindWorkflow
	case "correction":
		return asymptoteobserve.LearningMemoryKindCorrection
	default:
		return asymptoteobserve.LearningMemoryKindConvention
	}
}

func p4cToMem(c p4cCase, m p4cMemory) asymptoteobserve.LearningMemoryV1 {
	source := m.Source
	if source == "" {
		source = "verified"
	}
	return asymptoteobserve.LearningMemoryV1{
		ID: m.ID, CandidateID: "candidate-" + m.ID, Kind: p4cKind(m.Kind),
		Title: m.Title, Body: m.Body,
		Project:   asymptoteobserve.LearningProjectV1{ID: c.Project},
		Tags:      []string{"source:" + source},
		Evidence:  []asymptoteobserve.LearningEvidenceV1{{TraceID: "synthetic-phase4c", EventIDs: []string{m.ID}}},
		CreatedAt: m.TS, UpdatedAt: m.TS,
	}
}

func TestPhase4CSelect(t *testing.T) {
	if os.Getenv("BEACON_PHASE4C_SELECT") != "1" {
		t.Skip("set BEACON_PHASE4C_SELECT=1 to run the Phase 4C selection dump")
	}
	fixturePath := os.Getenv("BEACON_PHASE4C_FIXTURE")
	outPath := os.Getenv("BEACON_PHASE4C_OUT")
	if fixturePath == "" || outPath == "" {
		t.Fatal("BEACON_PHASE4C_FIXTURE and BEACON_PHASE4C_OUT are required")
	}
	k := 8
	if v := os.Getenv("BEACON_PHASE4C_K"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			k = n
		}
	}
	raw, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var fx p4cFixture
	if err := json.Unmarshal(raw, &fx); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}

	configs := map[string]TrustContextOptions{
		"semantic_only":     {},
		"plus_supersession": {EnableSupersession: true},
		"plus_trust":        {EnableTrust: true},
		"plus_conflict":     {EnableConflict: true},
		"plus_injection":    {EnableInjectionDefense: true},
		"plus_all":          {EnableSupersession: true, EnableTrust: true, EnableConflict: true, EnableInjectionDefense: true, RequireSufficiency: true, WithholdIfInsufficient: true},
	}

	type caseOut struct {
		Retrieved map[string][]string            `json:"retrieved"` // method -> ids
		Configs   map[string][]string            `json:"configs"`   // config -> selected ids
		Signals   map[string]TrustContextSignals `json:"signals"`
	}
	out := map[string]caseOut{}

	var embed Embedder = NewHashedEmbedder()
	if url := os.Getenv("BEACON_EMBED_URL"); url != "" {
		if e, err := NewHTTPEmbedder(url); err == nil {
			embed = e
		} else {
			fmt.Printf("P4C_NEURAL status=BLOCKED err=%q\n", err.Error())
		}
	}

	for _, c := range fx.Cases {
		byID := map[string]asymptoteobserve.LearningMemoryV1{}
		for _, m := range c.Memories {
			byID[m.ID] = p4cToMem(c, m)
		}
		store := OpenSemantic(filepath.Join(t.TempDir(), "sel.db"), embed)
		for _, m := range c.Memories {
			if err := store.PutMemory(byID[m.ID]); err != nil {
				t.Fatalf("put %s: %v", m.ID, err)
			}
		}
		res, err := store.SearchHybrid(Query{ProjectID: c.Project, Q: c.Query},
			HybridOptions{Limit: k, CandidateK: 50, LexicalWeight: 0, SemanticWeight: 1})
		if err != nil {
			t.Fatalf("search %s: %v", c.ID, err)
		}
		var retrieved []asymptoteobserve.LearningMemoryV1
		for _, r := range res {
			retrieved = append(retrieved, r.Memory)
		}
		co := caseOut{Retrieved: map[string][]string{}, Configs: map[string][]string{}, Signals: map[string]TrustContextSignals{}}
		ids := make([]string, 0, len(retrieved))
		for _, m := range retrieved {
			ids = append(ids, m.ID)
		}
		co.Retrieved["semantic"] = ids
		for name, opts := range configs {
			sel := SelectTrustContext(retrieved, c.Query, opts)
			var sids []string
			for _, m := range sel.Memories {
				sids = append(sids, m.ID)
			}
			co.Configs[name] = sids
			co.Signals[name] = sel.Signals
		}
		out[c.ID] = co
		_ = store.Close()
		fmt.Printf("P4C_SELECT case=%s retrieved=%v all=%v\n", c.ID, co.Retrieved["semantic"], co.Configs["plus_all"])
	}
	blob, _ := json.MarshalIndent(map[string]any{"k": k, "cases": out}, "", "  ")
	if err := os.WriteFile(outPath, append(blob, '\n'), 0o644); err != nil {
		t.Fatalf("write out: %v", err)
	}
}
