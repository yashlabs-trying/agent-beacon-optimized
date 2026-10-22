package learning

import (
	"github.com/asymptote-labs/agent-beacon/pkg/asymptoteobserve"
)

// This file adds context selection with a token budget. It is a pure function
// over ranked results: it never changes ranking or retrieval, only how much of
// the ranked list is handed to an agent. Provenance (evidence) is never dropped
// while its memory is kept; only the body text is truncated, and the memory is
// marked truncated.

// EstimateTokens approximates a token count for budgeting. It is a heuristic
// (about four characters per token) and deliberately dependency-free; it is
// used only to compare selection strategies, never as an exact tokenizer.
func EstimateTokens(s string) int {
	if s == "" {
		return 0
	}
	return (len([]rune(s)) + 3) / 4
}

// ContextOptions configures SelectContext. A zero field means "no limit".
type ContextOptions struct {
	MaxTokens          int
	MaxItems           int
	TruncateBodyTokens int
}

// ContextItem is one selected memory with its token cost.
type ContextItem struct {
	Memory    asymptoteobserve.LearningMemoryV1
	Tokens    int
	Truncated bool
}

// ContextSelection is the result of SelectContext.
type ContextSelection struct {
	Items       []ContextItem
	TotalTokens int
	// TotalUnbounded is the token cost of every input item untruncated.
	TotalUnbounded int
	Dropped        int
}

func truncateBody(body string, maxTokens int) (string, bool) {
	if maxTokens <= 0 {
		return body, false
	}
	runes := []rune(body)
	maxRunes := maxTokens * 4
	if len(runes) <= maxRunes {
		return body, false
	}
	return string(runes[:maxRunes]), true
}

// SelectContext walks the ranked results in order and includes items until the
// token budget or item cap is reached. Evidence is preserved for every included
// memory; only the body is truncated.
func SelectContext(results []HybridResult, opts ContextOptions) ContextSelection {
	var sel ContextSelection
	for _, r := range results {
		sel.TotalUnbounded += EstimateTokens(r.Memory.Title) + EstimateTokens(r.Memory.Body)
		if opts.MaxItems > 0 && len(sel.Items) >= opts.MaxItems {
			sel.Dropped++
			continue
		}
		titleTokens := EstimateTokens(r.Memory.Title)
		bodyLimit := opts.TruncateBodyTokens
		if opts.MaxTokens > 0 {
			remaining := opts.MaxTokens - sel.TotalTokens - titleTokens
			if remaining < 0 {
				sel.Dropped++
				continue
			}
			if bodyLimit == 0 || bodyLimit > remaining {
				bodyLimit = remaining
			}
		}
		body, truncated := r.Memory.Body, false
		if opts.MaxTokens > 0 && bodyLimit == 0 {
			body, truncated = "", body != ""
		} else {
			body, truncated = truncateBody(body, bodyLimit)
		}
		tokens := titleTokens + EstimateTokens(body)
		if opts.MaxTokens > 0 && sel.TotalTokens+tokens > opts.MaxTokens {
			sel.Dropped++
			continue
		}
		selected := r.Memory
		selected.Body = body
		sel.Items = append(sel.Items, ContextItem{Memory: selected, Tokens: tokens, Truncated: truncated})
		sel.TotalTokens += tokens
	}
	return sel
}

// SelectContextFromMemories is a convenience wrapper for a plain ranked slice.
func SelectContextFromMemories(memories []asymptoteobserve.LearningMemoryV1, opts ContextOptions) ContextSelection {
	results := make([]HybridResult, len(memories))
	for i, m := range memories {
		results[i] = HybridResult{Memory: m}
	}
	return SelectContext(results, opts)
}
