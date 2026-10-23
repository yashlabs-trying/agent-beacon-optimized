package learning

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// This is a historical regression diagnostic, never an independent V7 holdout.
// It is opt-in because it seeds one SQLite store per Phase 5 task.
func TestV7Phase5HistoricalDiagnostic(t *testing.T) {
	if os.Getenv("BEACON_V7_PHASE5_DIAGNOSTIC") != "1" {
		t.Skip("historical diagnostic is opt-in")
	}
	fixture, output := os.Getenv("BEACON_V7_FIXTURE"), os.Getenv("BEACON_V7_OUT")
	if fixture == "" || output == "" {
		t.Fatal("BEACON_V7_FIXTURE and BEACON_V7_OUT are required")
	}
	data, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	var fx struct {
		Tasks []struct {
			p5Task
			Request          string   `json:"request"`
			Category         string   `json:"category"`
			RequiredEvidence []string `json:"required_evidence"`
		} `json:"tasks"`
	}
	if err := json.Unmarshal(data, &fx); err != nil {
		t.Fatal(err)
	}
	type record struct {
		ID               string   `json:"id"`
		Category         string   `json:"category"`
		Selected         []string `json:"selected"`
		RequiredEvidence []string `json:"required_evidence"`
		Route            string   `json:"route"`
		Sufficient       bool     `json:"sufficient"`
		EmbeddingModel   string   `json:"embedding_model,omitempty"`
	}
	var records []record
	for _, task := range fx.Tasks {
		store := OpenSemantic(filepath.Join(t.TempDir(), "memory.db"), NewHashedEmbedder())
		for _, m := range task.Memories {
			if err := store.PutMemory(p5ToMem(task.p5Task, m)); err != nil {
				t.Fatalf("%s: %v", task.ID, err)
			}
		}
		result, err := store.SearchQuestion(Query{ProjectID: task.Project, Q: task.RetrievalQuery}, task.Request, QuestionOptions{})
		if err != nil {
			t.Fatalf("%s: %v", task.ID, err)
		}
		records = append(records, record{
			ID: task.ID, Category: task.Category, Selected: questionIDs(result),
			RequiredEvidence: task.RequiredEvidence, Route: result.Route,
			Sufficient: result.Signals.Sufficient, EmbeddingModel: result.EmbeddingModel,
		})
		if err := store.Close(); err != nil {
			t.Fatal(err)
		}
	}
	encoded, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(output, append(encoded, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}
