package learning

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/asymptote-labs/agent-beacon/pkg/asymptoteobserve"
)

// These labels are separate from the original 17-case set. Each case has one
// unique gold memory, so duplicate bodies cannot create ambiguous rankings.
var extendedNeuralCases = []semCase{
	{"paraphrase", "build used old packages after dependency manifest edit", "Clear the CI dependency cache after changing the lockfile to avoid stale packages."},
	{"paraphrase", "service keeps returning unauthorized after credential renewal", "Refresh the bearer token before retrying a request that returned HTTP 401."},
	{"paraphrase", "database writer waits forever during competing update", "Set a SQLite busy timeout before retrying a transaction under write contention."},
	{"paraphrase", "background worker survives request cancellation", "Cancel the worker context when its parent request completes to avoid a goroutine leak."},
	{"paraphrase", "container build still uses previous base layer", "Invalidate Docker layer cache after updating the base image digest."},
	{"paraphrase", "deleted rows still slow index scans", "Vacuum and analyze the Postgres table after bulk deletes create index bloat."},
	{"paraphrase", "integration test fails only when network briefly drops", "Retry a transient network fetch once after inspecting the package smoke test error."},
	{"paraphrase", "release archive lacks generated binary", "Build the hook executable before packaging the Beacon release archive."},
	{"paraphrase", "old schema is served after application restart", "Run the database migration before starting readers after a schema version change."},
	{"paraphrase", "HTTP client hangs after caller leaves", "Attach the caller context to every outbound HTTP request so cancellation propagates."},
	{"synonym", "prevent credential lifetime failures", "Rotate expiring API credentials and refresh the active session before the next authenticated call."},
	{"synonym", "stop duplicate work after message redelivery", "Use an idempotency key when processing a message that the queue may deliver twice."},
	{"synonym", "make deployment easy to undo", "Keep the previous container image digest available for a fast rollback."},
	{"synonym", "avoid inconsistent reads during a write", "Wrap related SQLite row and index updates in one transaction."},
	{"synonym", "limit runaway prompt size", "Trim low priority retrieved memories before exceeding the agent context budget."},
	{"synonym", "recover search after missing derived data", "Rebuild the FTS index from authoritative memory rows when the index table is missing."},
	{"synonym", "block outdated advice from retrieval", "Exclude superseded memories from search results after a replacement is approved."},
	{"synonym", "keep tenant data isolated", "Filter every memory query by the current project identifier before ranking."},
	{"synonym", "avoid partial writes during crash", "Commit the authoritative memory row and its generation counter atomically."},
	{"synonym", "detect semantic index drift", "Record the embedding model version beside each vector and rebuild mismatched vectors."},
	{"hard_negative", "retry a locked SQLite transaction, not a network call", "When SQLite reports database locked, wait for the busy timeout and retry the transaction."},
	{"hard_negative", "retry a failed network download, not a database transaction", "When a package download times out, retry the HTTP fetch once with backoff."},
	{"hard_negative", "invalidate only the memory search cache after an approval", "After approving a memory, increment the memory generation to invalidate cached search results."},
	{"hard_negative", "invalidate only the build cache after lockfile changes", "After changing package versions, clear the CI build cache to avoid stale artifacts."},
	{"hard_negative", "rotate the signing key without changing user passwords", "Rotate the JWT signing key and reissue tokens while leaving stored password hashes intact."},
	{"hard_negative", "reset a forgotten password without rotating the signing key", "Use the password reset workflow to replace a user's password hash without changing the JWT signing key."},
	{"hard_negative", "cancel a goroutine, not a SQL statement", "Close the worker context to stop the background goroutine when its request ends."},
	{"hard_negative", "cancel a SQL statement, not the worker process", "Apply a query timeout to the SQL statement while keeping the worker process alive."},
	{"hard_negative", "rebuild embeddings after model upgrade, not FTS", "Regenerate stored vectors when the neural embedding model revision changes."},
	{"hard_negative", "rebuild FTS after tokenizer change, not neural vectors", "Regenerate the full-text index when the lexical tokenizer configuration changes."},
	{"identifier", "CACHE_TTL_MS setting", "CACHE_TTL_MS controls the local search cache lifetime in milliseconds."},
	{"identifier", "AUTH_RETRY_MAX setting", "AUTH_RETRY_MAX limits the number of authentication retries before failing."},
	{"identifier", "BEACON_EMBED_URL variable", "BEACON_EMBED_URL points the memory engine to the local embedding HTTP sidecar."},
	{"identifier", "FTS_REBUILD_BATCH constant", "FTS_REBUILD_BATCH sets the number of memory rows indexed per rebuild batch."},
	{"identifier", "sqlite error SQLITE_BUSY", "SQLITE_BUSY means another connection currently holds a conflicting database lock."},
	{"identifier", "HTTP status 429", "HTTP 429 indicates rate limiting and should honor the Retry-After header."},
	{"identifier", "field superseded_by", "The superseded_by field identifies the approved replacement for an older memory."},
	{"identifier", "function RebuildEmbeddings", "RebuildEmbeddings refreshes stored vectors from current approved memory text."},
	{"identifier", "file go.sum", "The go.sum file records dependency checksums used by Go module verification."},
	{"identifier", "header X-Request-ID", "The X-Request-ID header carries a request correlation identifier across services."},
}

func TestNeuralExtendedQuality(t *testing.T) {
	url := os.Getenv("BEACON_EMBED_URL")
	if url == "" {
		t.Skip("set BEACON_EMBED_URL to run the 40-case neural comparison")
	}
	neural, err := NewHTTPEmbedder(url)
	if err != nil {
		t.Fatal(err)
	}
	makeStore := func(name string, e Embedder) *Store {
		t.Helper()
		store := OpenSemantic(filepath.Join(t.TempDir(), name+".db"), e)
		for i, c := range extendedNeuralCases {
			m := asymptoteobserve.LearningMemoryV1{
				ID: fmt.Sprintf("extended-%02d", i), CandidateID: fmt.Sprintf("candidate-%02d", i),
				Kind:  asymptoteobserve.LearningMemoryKindDebuggingPattern,
				Title: fmt.Sprintf("lesson-%02d", i), Body: c.gold,
				Project:  asymptoteobserve.LearningProjectV1{ID: "proj-a"},
				Evidence: []asymptoteobserve.LearningEvidenceV1{{TraceID: fmt.Sprintf("trace-%02d", i), EventIDs: []string{"e1"}}},
			}
			if err := store.PutMemory(m); err != nil {
				t.Fatal(err)
			}
		}
		return store
	}
	hashedStore := makeStore("hashed", NewHashedEmbedder())
	neuralStore := makeStore("neural", neural)

	type stats struct {
		n           int
		r1, r5, mrr float64
		times       []time.Duration
	}
	for _, method := range []struct {
		name     string
		store    *Store
		embedder Embedder
	}{
		{"lexical", hashedStore, nil},
		{"hashed", hashedStore, NewHashedEmbedder()},
		{"neural", neuralStore, neural},
	} {
		byCategory := map[string]*stats{}
		for i, c := range extendedNeuralCases {
			begin := time.Now()
			var got []string
			if method.embedder == nil {
				memories, err := method.store.ListMemories(Query{ProjectID: "proj-a", Q: c.query, Limit: 5})
				if err != nil {
					t.Fatal(err)
				}
				for _, m := range memories {
					got = append(got, m.ID)
				}
			} else {
				db, err := method.store.db()
				if err != nil {
					t.Fatal(err)
				}
				hits, searchErr := method.store.semanticSearchExact(db, Query{ProjectID: "proj-a"}, method.embedder.Embed(c.query), 5)
				if searchErr != nil {
					t.Fatal(searchErr)
				}
				for _, h := range hits {
					got = append(got, h.memory.ID)
				}
			}
			elapsed := time.Since(begin)
			r1, r5, mrr, _ := evaluateRanking(got, fmt.Sprintf("extended-%02d", i))
			s := byCategory[c.category]
			if s == nil {
				s = &stats{}
				byCategory[c.category] = s
			}
			s.n++
			s.r1 += r1
			s.r5 += r5
			s.mrr += mrr
			s.times = append(s.times, elapsed)
		}
		for _, category := range []string{"paraphrase", "synonym", "hard_negative", "identifier"} {
			s := byCategory[category]
			sort.Slice(s.times, func(i, j int) bool { return s.times[i] < s.times[j] })
			fmt.Printf("EXTENDED method=%s category=%s n=%d recall@1=%.3f recall@5=%.3f mrr=%.3f p50_ms=%.2f p95_ms=%.2f\n",
				method.name, category, s.n, s.r1/float64(s.n), s.r5/float64(s.n), s.mrr/float64(s.n),
				float64(s.times[len(s.times)/2].Microseconds())/1000, float64(s.times[len(s.times)-1].Microseconds())/1000)
		}
	}
}
