// phase1seed prepares disposable project-scoped memory databases for the
// local coding-agent comparison. It is kept under testdata so shipping builds
// do not include this evaluation utility.
package main

import (
	"fmt"
	"os"

	"github.com/asymptote-labs/agent-beacon/cli/beacon/internal/learning"
	"github.com/asymptote-labs/agent-beacon/pkg/asymptoteobserve"
)

func main() {
	if len(os.Args) != 4 {
		fmt.Fprintln(os.Stderr, "usage: phase1seed <task> <workspace> <database>")
		os.Exit(2)
	}
	project, err := learning.ResolveProject(os.Args[2])
	if err != nil {
		panic(err)
	}
	store := learning.Open(os.Args[3])
	memories := map[string][]asymptoteobserve.LearningMemoryV1{
		"feature": {
			{ID: "feature-current", Title: "Feature flag parsing", Body: "For feature environment flags, trim whitespace and compare case-insensitively. Only 1, true, yes, and on mean enabled; 0, false, no, off, and unset mean disabled."},
			{ID: "feature-distractor", Title: "Feature deployment", Body: "Deploy feature flag changes through the normal review pipeline."},
		},
		"retry": {
			{ID: "retry-current", Title: "Retry delay convention", Body: "Retry attempts are numbered from one. Delay is min(base * 2^(attempt-1), cap); attempts below one are invalid."},
			{ID: "retry-distractor", Title: "Retry network fetch", Body: "Retry a transient package download once after checking its error."},
		},
		"slug": {
			{ID: "slug-old", Title: "Old slug convention", Body: "Use lowercase words joined by hyphens for generated slugs.", SupersededBy: "slug-current"},
			{ID: "slug-current", Title: "Current slug convention", Body: "Generated slugs use lowercase ASCII letters and digits joined by a single underscore. Strip separators at both ends; this replaces the old hyphen rule."},
			{ID: "slug-distractor", Title: "URL slug", Body: "External URL slugs may still use hyphens; the local generated slug function uses underscores."},
		},
	}
	selected, ok := memories[os.Args[1]]
	if !ok {
		fmt.Fprintln(os.Stderr, "unknown task")
		os.Exit(2)
	}
	for _, m := range selected {
		m.CandidateID = "candidate-" + m.ID
		m.Kind = asymptoteobserve.LearningMemoryKindConvention
		m.Project = project
		m.Evidence = []asymptoteobserve.LearningEvidenceV1{{TraceID: "synthetic-phase1", EventIDs: []string{"e1"}}}
		if err := store.PutMemory(m); err != nil {
			panic(err)
		}
	}
	fmt.Printf("seeded %d memories for %s, project %s\n", len(selected), os.Args[1], project.ID)
}
