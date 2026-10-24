// phase5seed prepares a disposable project-scoped memory database for one Phase 5 task.
//
//	phase5seed <task-id> <workspace> <database> <fixture.json>
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

type task struct {
	ID       string   `json:"id"`
	Project  string   `json:"project"`
	Memories []memory `json:"memories"`
}

type fixture struct {
	Tasks []task `json:"tasks"`
}

func kindOf(k string) string {
	switch k {
	case "gotcha":
		return asymptoteobserve.LearningMemoryKindGotcha
	case "workflow":
		return asymptoteobserve.LearningMemoryKindWorkflow
	case "correction":
		return asymptoteobserve.LearningMemoryKindCorrection
	case "decision":
		return asymptoteobserve.LearningMemoryKindConvention
	default:
		return asymptoteobserve.LearningMemoryKindConvention
	}
}

func main() {
	if len(os.Args) != 5 {
		fmt.Fprintln(os.Stderr, "usage: phase5seed <task-id> <workspace> <database> <fixture.json>")
		os.Exit(2)
	}
	taskID, workspace, dbPath, fixturePath := os.Args[1], os.Args[2], os.Args[3], os.Args[4]
	raw, err := os.ReadFile(fixturePath)
	if err != nil {
		panic(err)
	}
	var fx fixture
	if err := json.Unmarshal(raw, &fx); err != nil {
		panic(err)
	}
	var t *task
	for i := range fx.Tasks {
		if fx.Tasks[i].ID == taskID {
			t = &fx.Tasks[i]
			break
		}
	}
	if t == nil {
		fmt.Fprintf(os.Stderr, "unknown task %q\n", taskID)
		os.Exit(2)
	}
	project, err := learning.ResolveProject(workspace)
	if err != nil {
		panic(err)
	}
	store := learning.Open(dbPath)
	defer store.Close()
	for _, mm := range t.Memories {
		source := mm.Source
		if source == "" {
			source = "verified"
		}
		mem := asymptoteobserve.LearningMemoryV1{
			ID: mm.ID, CandidateID: "candidate-" + mm.ID, Kind: kindOf(mm.Kind),
			Title: mm.Title, Body: mm.Body, Project: project,
			Tags:      []string{"source:" + source},
			Evidence:  []asymptoteobserve.LearningEvidenceV1{{TraceID: "synthetic-phase5", EventIDs: []string{mm.ID}}},
			CreatedAt: mm.TS, UpdatedAt: mm.TS,
		}
		if err := store.PutMemory(mem); err != nil {
			panic(err)
		}
	}
	fmt.Printf("seeded %d memories for %s, project %s\n", len(t.Memories), taskID, project.ID)
}
