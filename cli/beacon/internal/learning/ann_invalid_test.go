package learning

import (
	"math"
	"testing"
)

func TestANNRejectsInvalidVectors(t *testing.T) {
	idx := newANNIndex(defaultANNParams())
	idx.add("valid-a", []float32{1, 0, 0})
	idx.add("zero", []float32{0, 0, 0})
	idx.add("nan", []float32{float32(math.NaN()), 0, 0})
	idx.add("inf", []float32{float32(math.Inf(1)), 0, 0})
	idx.add("wrong-dim", []float32{1, 0})
	idx.add("valid-b", []float32{0, 1, 0})
	if idx.len() != 2 {
		t.Fatalf("ANN indexed %d nodes; want only the two finite nonzero 3D vectors", idx.len())
	}
	if got := idx.search([]float32{1, 0, 0}, 1); len(got) != 1 || got[0] != "valid-a" {
		t.Fatalf("valid query returned %v", got)
	}
	for _, invalid := range [][]float32{
		{0, 0, 0}, {float32(math.NaN()), 0, 0}, {1, 0},
	} {
		if got := idx.search(invalid, 1); len(got) != 0 {
			t.Fatalf("invalid query returned %v", got)
		}
	}
}
