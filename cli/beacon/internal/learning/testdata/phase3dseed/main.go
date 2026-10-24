// phase3dseed prepares disposable project-scoped memory databases for the Phase 3
// six-task paired agent evaluation. It reads the locked fixture
// reports/phase3/heldout/agent_tasks_v1.json and inserts each task's approved
// note and distractor note as approved memories scoped to the workspace project.
//
// It is kept under testdata so shipping builds do not include this evaluation
// utility. Usage:
//
//	phase3dseed <task-id> <workspace> <database> <fixture.json>
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/asymptote-labs/agent-beacon/cli/beacon/internal/learning"
	"github.com/asymptote-labs/agent-beacon/pkg/asymptoteobserve"
)

type fixtureTask struct {
	ID             string `json:"id"`
	ApprovedNote   string `json:"approved_note"`
	DistractorNote string `json:"distractor_note"`
}

type fixture struct {
	Tasks []fixtureTask `json:"tasks"`
}

func main() {
	if len(os.Args) != 5 {
		fmt.Fprintln(os.Stderr, "usage: phase3dseed <task-id> <workspace> <database> <fixture.json>")
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
	var task *fixtureTask
	for i := range fx.Tasks {
		if fx.Tasks[i].ID == taskID {
			task = &fx.Tasks[i]
			break
		}
	}
	if task == nil {
		fmt.Fprintf(os.Stderr, "unknown task %q\n", taskID)
		os.Exit(2)
	}

	project, err := learning.ResolveProject(workspace)
	if err != nil {
		panic(err)
	}
	store := learning.Open(dbPath)
	defer store.Close()

	memories := []asymptoteobserve.LearningMemoryV1{
		{
			ID: taskID + "-current", Title: taskID + " approved note", Body: task.ApprovedNote,
			CandidateID: "candidate-" + taskID + "-current",
			Kind:        asymptoteobserve.LearningMemoryKindConvention,
			Project:     project,
			Evidence:    []asymptoteobserve.LearningEvidenceV1{{TraceID: "synthetic-phase3d", EventIDs: []string{"e1"}}},
		},
		{
			ID: taskID + "-distractor", Title: taskID + " distractor note", Body: task.DistractorNote,
			CandidateID: "candidate-" + taskID + "-distractor",
			Kind:        asymptoteobserve.LearningMemoryKindConvention,
			Project:     project,
			Evidence:    []asymptoteobserve.LearningEvidenceV1{{TraceID: "synthetic-phase3d", EventIDs: []string{"e2"}}},
		},
	}
	for _, m := range memories {
		if err := store.PutMemory(m); err != nil {
			panic(err)
		}
	}
	fmt.Printf("seeded %d memories for %s, project %s\n", len(memories), taskID, project.ID)
}
