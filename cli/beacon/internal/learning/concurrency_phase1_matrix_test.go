package learning

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/asymptote-labs/agent-beacon/pkg/asymptoteobserve"
)

func TestConcurrencyPhase1Matrix(t *testing.T) {
	if os.Getenv("BEACON_CONCURRENCY_MATRIX") != "1" {
		t.Skip("set BEACON_CONCURRENCY_MATRIX=1 for bounded client sweep")
	}
	const operations = 25
	project := asymptoteobserve.LearningProjectV1{ID: "phase1-a"}
	for _, clients := range []int{1, 4, 8, 16} {
		for _, mode := range []string{"read", "write", "mixed", "supersede", "rebuild"} {
			t.Run(fmt.Sprintf("%s-%d", mode, clients), func(t *testing.T) {
				store := Open(filepath.Join(t.TempDir(), "memory.db"))
				for i := 0; i < 100; i++ {
					if err := store.PutMemory(asymptoteobserve.LearningMemoryV1{
						ID: fmt.Sprintf("base-%03d", i), CandidateID: fmt.Sprintf("base-c-%03d", i),
						Kind:  asymptoteobserve.LearningMemoryKindConvention,
						Title: "base", Body: "needle concurrency memory",
						Project: project,
					}); err != nil {
						t.Fatal(err)
					}
				}
				if err := store.PutMemory(asymptoteobserve.LearningMemoryV1{
					ID: "foreign", CandidateID: "foreign-c", Kind: asymptoteobserve.LearningMemoryKindConvention,
					Title: "foreign", Body: "needle concurrency memory",
					Project: asymptoteobserve.LearningProjectV1{ID: "phase1-b"},
				}); err != nil {
					t.Fatal(err)
				}
				if mode == "supersede" {
					for worker := 0; worker < clients; worker++ {
						for op := 0; op < operations; op++ {
							id := fmt.Sprintf("old-%02d-%02d", worker, op)
							if err := store.PutMemory(asymptoteobserve.LearningMemoryV1{
								ID: id, CandidateID: "c-" + id, Kind: asymptoteobserve.LearningMemoryKindConvention,
								Title: "old", Body: "needle supersession", Project: project,
							}); err != nil {
								t.Fatal(err)
							}
						}
					}
				}
				if _, err := store.ListMemories(Query{ProjectID: project.ID, Q: "needle", Limit: 5}); err != nil {
					t.Fatal(err)
				}
				gate := make(chan struct{})
				latencies := make(chan time.Duration, clients*operations)
				errors := make(chan error, clients*operations)
				var workers sync.WaitGroup
				for worker := 0; worker < clients; worker++ {
					workers.Add(1)
					go func(worker int) {
						defer workers.Done()
						<-gate
						for op := 0; op < operations; op++ {
							begin := time.Now()
							var err error
							switch mode {
							case "write", "mixed":
								if mode == "write" || op%2 == 0 {
									id := fmt.Sprintf("write-%02d-%02d", worker, op)
									err = store.PutMemory(asymptoteobserve.LearningMemoryV1{
										ID: id, CandidateID: "c-" + id, Kind: asymptoteobserve.LearningMemoryKindConvention,
										Title: "new", Body: "needle write memory", Project: project,
									})
								} else {
									err = phase1Read(store, project.ID)
								}
							case "supersede":
								id := fmt.Sprintf("old-%02d-%02d", worker, op)
								err = store.PutMemory(asymptoteobserve.LearningMemoryV1{
									ID: id, CandidateID: "c-" + id, Kind: asymptoteobserve.LearningMemoryKindConvention,
									Title: "old", Body: "needle supersession", Project: project, SupersededBy: "replacement",
								})
							case "rebuild":
								if worker == 0 && op == 0 {
									var dbErr error
									db, dbErr := store.db()
									if dbErr == nil {
										_, dbErr = db.Exec(`UPDATE memory_index_meta SET value = '0' WHERE key = 'index_ready'`)
									}
									if dbErr != nil {
										err = dbErr
									}
								}
								if err == nil {
									err = phase1Read(store, project.ID)
								}
							default:
								err = phase1Read(store, project.ID)
							}
							latencies <- time.Since(begin)
							if err != nil {
								errors <- err
							}
						}
					}(worker)
				}
				start := time.Now()
				close(gate)
				workers.Wait()
				elapsed := time.Since(start)
				close(latencies)
				close(errors)
				ordered := make([]time.Duration, 0, clients*operations)
				for latency := range latencies {
					ordered = append(ordered, latency)
				}
				sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
				failures := len(errors)
				fmt.Printf("CONCURRENCY mode=%s clients=%d ops=%d throughput=%.1f/s errors=%d p95_ms=%.2f p99_ms=%.2f peak_rss_mb=%.1f\n",
					mode, clients, len(ordered), float64(len(ordered))/elapsed.Seconds(), failures,
					float64(annPercentile(ordered, 0.95).Microseconds())/1000,
					float64(annPercentile(ordered, 0.99).Microseconds())/1000,
					float64(peakRSSKB())/1024)
				if failures > 0 {
					t.Fatalf("%d errors; first: %v", failures, <-errors)
				}
				if mode == "supersede" {
					got, err := store.ListMemories(Query{ProjectID: project.ID, Q: "supersession", Limit: clients*operations + 5})
					if err != nil || len(got) != 0 {
						t.Fatalf("superseded results=%d error=%v", len(got), err)
					}
				}
			})
		}
	}
}

func phase1Read(store *Store, projectID string) error {
	got, err := store.ListMemories(Query{ProjectID: projectID, Q: "needle", Limit: 5})
	if err != nil {
		return err
	}
	if len(got) == 0 {
		return fmt.Errorf("empty search")
	}
	for _, m := range got {
		if m.Project.ID != projectID || m.SupersededBy != "" {
			return fmt.Errorf("unauthorized or superseded result %s", m.ID)
		}
	}
	return nil
}
