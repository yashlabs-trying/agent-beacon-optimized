package learning

import (
	"hash/fnv"
	"math"
	"strings"
	"unicode"
)

// Embedder turns text into a fixed-length, L2-normalized vector. It is the seam
// for the optional semantic layer: the default build ships a dependency-free
// local implementation, and a neural model can be added behind the same
// interface without touching storage or fusion.
type Embedder interface {
	// Model identifies the embedding model and version. It is stored with every
	// vector so a model change invalidates only the vectors it produced.
	Model() string
	// Dim is the vector length. Vectors of a different length are never compared.
	Dim() int
	// Embed returns an L2-normalized vector for text.
	Embed(text string) []float32
}

// HashedEmbedder is a local, dependency-free embedder using signed feature
// hashing over word tokens and character trigrams.
//
// It is deliberately modest and honestly labelled: it captures morphology and
// partial lexical overlap (so "expiry"/"expired"/"expire" and shared technical
// terms score as similar) but it is NOT a neural model and does not capture true
// synonymy ("JWT refresh failures" vs "authentication token expiry" share few
// features). The interface is the point; a neural backend can replace it without
// changing callers.
type HashedEmbedder struct {
	dim   int
	model string
}

// NewHashedEmbedder returns the default local embedder.
func NewHashedEmbedder() *HashedEmbedder {
	return &HashedEmbedder{dim: 512, model: "hashed-subword-v1"}
}

func (e *HashedEmbedder) Model() string { return e.model }
func (e *HashedEmbedder) Dim() int      { return e.dim }

// Embed implements Embedder.
func (e *HashedEmbedder) Embed(text string) []float32 {
	vec := make([]float32, e.dim)
	lower := strings.ToLower(text)
	for _, word := range embedWords(lower) {
		embedAdd(vec, "w:"+word, 1.0)
		// Character trigrams with word boundaries give morphological overlap.
		padded := "^" + word + "$"
		runes := []rune(padded)
		for i := 0; i+3 <= len(runes); i++ {
			embedAdd(vec, "g:"+string(runes[i:i+3]), 0.5)
		}
	}
	var norm float64
	for _, x := range vec {
		norm += float64(x) * float64(x)
	}
	if norm > 0 {
		inv := float32(1 / math.Sqrt(norm))
		for i := range vec {
			vec[i] *= inv
		}
	}
	return vec
}

func embedWords(text string) []string {
	return strings.FieldsFunc(text, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}

func embedAdd(vec []float32, feature string, weight float32) {
	h := fnv.New32a()
	_, _ = h.Write([]byte(feature))
	sum := h.Sum32()
	sign := float32(1)
	if sum&1 == 1 {
		sign = -1
	}
	idx := int((sum >> 1) % uint32(len(vec)))
	vec[idx] += sign * weight
}

// cosine returns the cosine similarity of two L2-normalized vectors, or 0 if
// their lengths differ.
func cosine(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}
	var dot float32
	for i := range a {
		dot += a[i] * b[i]
	}
	return dot
}
