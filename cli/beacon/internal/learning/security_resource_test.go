package learning

import (
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/asymptote-labs/agent-beacon/pkg/asymptoteobserve"
)

// TestSearchScanCapReturnsExplicitError proves the non-selective text scan is
// bounded and fails loudly when the cap is exceeded, rather than silently
// truncating or scanning without bound.
func TestSearchScanCapReturnsExplicitError(t *testing.T) {
	old := searchScanRowCap
	searchScanRowCap = 3
	defer func() { searchScanRowCap = old }()

	store := Open(filepath.Join(scratchDirTB(t), "memory.db"))
	defer store.Close()
	for i := 0; i < 10; i++ {
		if err := store.PutMemory(asymptoteobserve.LearningMemoryV1{
			ID:          fmt.Sprintf("m-%d", i),
			CandidateID: fmt.Sprintf("c-%d", i),
			Kind:        asymptoteobserve.LearningMemoryKindConvention,
			Title:       "Convention",
			Body:        "Body text.",
			Project:     asymptoteobserve.LearningProjectV1{ID: "p"},
		}); err != nil {
			t.Fatal(err)
		}
	}
	// "ab" is a two-character term, so the planner takes the scan path.
	_, err := store.ListMemories(Query{ProjectID: "p", Q: "ab"})
	if !errors.Is(err, ErrSearchScanExceeded) {
		t.Fatalf("err = %v, want ErrSearchScanExceeded", err)
	}
}

// TestSelectiveQueryStaysUnderCap confirms a selective query is answered by the
// indexed path and does not trip the scan cap.
func TestSelectiveQueryStaysUnderCap(t *testing.T) {
	old := searchScanRowCap
	searchScanRowCap = 3
	defer func() { searchScanRowCap = old }()

	store := Open(filepath.Join(scratchDirTB(t), "memory.db"))
	defer store.Close()
	for i := 0; i < 10; i++ {
		body := "ordinary workflow text"
		if i == 7 {
			body = "the rare unique-needle recovery step"
		}
		if err := store.PutMemory(asymptoteobserve.LearningMemoryV1{
			ID:          fmt.Sprintf("m-%d", i),
			CandidateID: fmt.Sprintf("c-%d", i),
			Kind:        asymptoteobserve.LearningMemoryKindConvention,
			Title:       "Convention",
			Body:        body,
			Project:     asymptoteobserve.LearningProjectV1{ID: "p"},
		}); err != nil {
			t.Fatal(err)
		}
	}
	got, err := store.ListMemories(Query{ProjectID: "p", Q: "unique-needle", Limit: 5})
	if err != nil {
		t.Fatalf("selective query errored: %v", err)
	}
	if len(got) != 1 || got[0].ID != "m-7" {
		t.Fatalf("selective query = %#v, want m-7", got)
	}
}
