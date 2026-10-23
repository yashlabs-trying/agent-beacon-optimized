// phase4cseed prepares disposable project-scoped memory databases for the Phase 4C
// intelligent-memory benchmark. It reads a Phase 4C fixture and inserts every memory
// for one case, scoped to the workspace project, preserving provenance tags and
// timestamps so the trust/supersession layer has real signals.
//
//	phase4cseed <case-id> <workspace> <database> <fixture.json>
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/asymptote-labs/agent-beacon/cli/beacon/internal/learning"
	"github.com/asymptote-labs/agent-beacon/pkg/asymptoteobserve"
)

type memory struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Body   string `json:"body"`
	Kind   string `json:"kind"`
	TS     string `json:"ts"`
	Source string `json:"source"`
}

type fixtureCase struct {
	ID       string   `json:"id"`
	Project  string   `json:"project"`
	Memories []memory `json:"memories"`
}

type fixture struct {
	Cases []fixtureCase `json:"cases"`
}

func kindOf(k string) string {
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

func main() {
	if len(os.Args) != 5 {
		fmt.Fprintln(os.Stderr, "usage: phase4cseed <case-id> <workspace> <database> <fixture.json>")
		os.Exit(2)
	}
	caseID, workspace, dbPath, fixturePath := os.Args[1], os.Args[2], os.Args[3], os.Args[4]
	raw, err := os.ReadFile(fixturePath)
	if err != nil {
		panic(err)
	}
	var fx fixture
	if err := json.Unmarshal(raw, &fx); err != nil {
		panic(err)
	}
	var c *fixtureCase
	for i := range fx.Cases {
		if fx.Cases[i].ID == caseID {
			c = &fx.Cases[i]
			break
		}
	}
	if c == nil {
		fmt.Fprintf(os.Stderr, "unknown case %q\n", caseID)
		os.Exit(2)
	}
	project, err := learning.ResolveProject(workspace)
	if err != nil {
		panic(err)
	}
	store := learning.Open(dbPath)
	defer store.Close()
	for _, m := range c.Memories {
		source := m.Source
		if source == "" {
			source = "verified"
		}
		mem := asymptoteobserve.LearningMemoryV1{
			ID: m.ID, CandidateID: "candidate-" + m.ID, Kind: kindOf(m.Kind),
			Title: m.Title, Body: m.Body, Project: project,
			Tags:      []string{"source:" + source},
			Evidence:  []asymptoteobserve.LearningEvidenceV1{{TraceID: "synthetic-phase4c", EventIDs: []string{m.ID}}},
			CreatedAt: m.TS, UpdatedAt: m.TS,
		}
		if err := store.PutMemory(mem); err != nil {
			panic(err)
		}
	}
	fmt.Printf("seeded %d memories for %s, project %s\n", len(c.Memories), caseID, project.ID)
}
