package learning

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// HTTPEmbedder calls a local embedding sidecar (see embed_sidecar.py), typically
// a sentence-transformers model on a GPU. It is opt-in: Open still has no
// embedder, and nothing calls this unless the caller constructs it. The sidecar
// is local-only and runs offline after the model is cached; no external API is
// contacted.
type HTTPEmbedder struct {
	url    string
	model  string
	dim    int
	client *http.Client
}

// NewHTTPEmbedder probes the sidecar and records its model id and dimension.
func NewHTTPEmbedder(baseURL string) (*HTTPEmbedder, error) {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" {
		return nil, fmt.Errorf("empty embed sidecar url")
	}
	e := &HTTPEmbedder{url: base, client: &http.Client{Timeout: 120 * time.Second}}
	resp, err := e.client.Get(base + "/health")
	if err != nil {
		return nil, fmt.Errorf("embed sidecar unreachable at %s: %w", base, err)
	}
	defer resp.Body.Close()
	var info struct {
		Model string `json:"model"`
		Dim   int    `json:"dim"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("decode sidecar health: %w", err)
	}
	if info.Dim <= 0 {
		return nil, fmt.Errorf("sidecar reported dim %d", info.Dim)
	}
	e.model, e.dim = info.Model, info.Dim
	return e, nil
}

func (e *HTTPEmbedder) Model() string { return e.model }
func (e *HTTPEmbedder) Dim() int      { return e.dim }

// Embed implements Embedder for a single string.
func (e *HTTPEmbedder) Embed(text string) []float32 {
	vs := e.EmbedBatch([]string{text})
	if len(vs) == 0 {
		return nil
	}
	return vs[0]
}

// EmbedBatch embeds several strings in one request.
func (e *HTTPEmbedder) EmbedBatch(texts []string) [][]float32 {
	if len(texts) == 0 {
		return nil
	}
	body, err := json.Marshal(map[string]any{"texts": texts})
	if err != nil {
		return nil
	}
	resp, err := e.client.Post(e.url+"/embed", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	var out struct {
		Vectors [][]float32 `json:"vectors"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil
	}
	return out.Vectors
}
