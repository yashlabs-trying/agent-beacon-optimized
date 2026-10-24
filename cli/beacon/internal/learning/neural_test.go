package learning

import (
	"fmt"
	"os"
	"testing"
)

func evaluateEmbedder(t *testing.T, e Embedder) (float64, float64, float64, float64) {
	t.Helper()
	store, cases := buildSemanticStoreWith(t, e)
	db, err := store.db()
	if err != nil {
		t.Fatal(err)
	}
	var r1, r5, mrr, ndcg float64
	for i, c := range cases {
		qvec := e.Embed(c.query)
		hits, err := store.semanticSearchExact(db, Query{ProjectID: "proj-a"}, qvec, 5)
		if err != nil {
			t.Fatal(err)
		}
		got := make([]string, len(hits))
		for j, h := range hits {
			got[j] = h.memory.ID
		}
		a, b, c2, d := evaluateRanking(got, goldID(cases, i))
		r1 += a
		r5 += b
		mrr += c2
		ndcg += d
	}
	n := float64(len(cases))
	return r1 / n, r5 / n, mrr / n, ndcg / n
}

// TestNeuralVsHashedQuality compares the hashed embedder against a real neural
// embedding sidecar on the labeled held-out set. It requires a running sidecar;
// set BEACON_EMBED_URL (e.g. http://127.0.0.1:8077).
func TestNeuralVsHashedQuality(t *testing.T) {
	url := os.Getenv("BEACON_EMBED_URL")
	if url == "" {
		t.Skip("set BEACON_EMBED_URL to run the neural comparison")
	}
	neural, err := NewHTTPEmbedder(url)
	if err != nil {
		t.Fatal(err)
	}
	hr1, hr5, hmrr, hndcg := evaluateEmbedder(t, NewHashedEmbedder())
	nr1, nr5, nmrr, nndcg := evaluateEmbedder(t, neural)
	fmt.Printf("NEURAL model=%s dim=%d\n", neural.Model(), neural.Dim())
	fmt.Printf("NEURAL hashed recall@1=%.3f recall@5=%.3f mrr=%.3f ndcg@5=%.3f\n", hr1, hr5, hmrr, hndcg)
	fmt.Printf("NEURAL neural recall@1=%.3f recall@5=%.3f mrr=%.3f ndcg@5=%.3f\n", nr1, nr5, nmrr, nndcg)
}
