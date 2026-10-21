package learning

import (
	"sort"
	"strings"
	"time"

	"github.com/asymptote-labs/agent-beacon/pkg/asymptoteobserve"
)

// This file adds a deterministic evidence-trust layer over retrieved memories.
// It never changes what is retrieved or who may see it; it annotates results
// with an evidence status and flags likely contradictions. Human approval is
// unchanged: a memory is still only present because it was approved, and this
// layer never approves anything.
//
// Everything here is deterministic and rule-based. There is no model call and
// no calibrated probability: the statuses are labels, not confidence scores.

// TrustStatus describes the evidence status of a memory, independent of its
// approval state.
type TrustStatus string

const (
	// TrustVerified means the memory carries at least one piece of usable
	// provenance and is within the freshness window.
	TrustVerified TrustStatus = "verified"
	// TrustUnverified means the memory has no usable supporting evidence.
	TrustUnverified TrustStatus = "unverified"
	// TrustStale means the memory has evidence but is older than the freshness
	// window. It remains usable; the label is a caution, not a rejection.
	TrustStale TrustStatus = "stale"
	// TrustContradicted means a deterministic rule found a likely conflict with
	// another applicable memory.
	TrustContradicted TrustStatus = "contradicted"
)

// TrustOptions configures AnnotateTrust.
type TrustOptions struct {
	// Now is the reference time. Zero means time.Now().UTC().
	Now time.Time
	// MaxAgeDays marks evidence older than this as stale. Zero disables the
	// freshness check.
	MaxAgeDays int
	// ContradictionTerms is the minimum number of shared significant terms for
	// two memories to be considered a contradiction candidate. Zero means 2.
	ContradictionTerms int
}

// TrustedMemory is a memory with its derived evidence status.
type TrustedMemory struct {
	Memory  asymptoteobserve.LearningMemoryV1
	Status  TrustStatus
	Reasons []string
}

// Contradiction is a pair of memories a deterministic rule judged to conflict,
// with the terms they share.
type Contradiction struct {
	A          asymptoteobserve.LearningMemoryV1
	B          asymptoteobserve.LearningMemoryV1
	SharedTerm []string
}

// AnnotateTrust labels each memory's evidence status and marks both sides of any
// detected contradiction. The input is not modified.
func AnnotateTrust(memories []asymptoteobserve.LearningMemoryV1, opts TrustOptions) []TrustedMemory {
	if opts.Now.IsZero() {
		opts.Now = time.Now().UTC()
	}
	if opts.ContradictionTerms <= 0 {
		opts.ContradictionTerms = 2
	}
	out := make([]TrustedMemory, len(memories))
	for i, m := range memories {
		out[i] = TrustedMemory{Memory: m, Status: evidenceStatus(m, opts), Reasons: evidenceReasons(m, opts)}
	}

	contradictions := detectContradictions(memories, opts)
	contradicted := map[string][]string{}
	for _, c := range contradictions {
		contradicted[c.A.ID] = c.SharedTerm
		contradicted[c.B.ID] = c.SharedTerm
	}
	for i := range out {
		if terms, ok := contradicted[out[i].Memory.ID]; ok {
			out[i].Status = TrustContradicted
			out[i].Reasons = append(out[i].Reasons, "conflicts with another applicable memory on: "+strings.Join(terms, ", "))
		}
	}
	return out
}

func evidenceStatus(m asymptoteobserve.LearningMemoryV1, opts TrustOptions) TrustStatus {
	if !hasUsableEvidence(m) {
		return TrustUnverified
	}
	if opts.MaxAgeDays > 0 && isStale(m, opts.Now, opts.MaxAgeDays) {
		return TrustStale
	}
	return TrustVerified
}

func evidenceReasons(m asymptoteobserve.LearningMemoryV1, opts TrustOptions) []string {
	var reasons []string
	if !hasUsableEvidence(m) {
		reasons = append(reasons, "no supporting evidence recorded")
	}
	if opts.MaxAgeDays > 0 && isStale(m, opts.Now, opts.MaxAgeDays) {
		reasons = append(reasons, "older than the freshness window")
	}
	if m.SupersededBy != "" {
		reasons = append(reasons, "superseded by "+m.SupersededBy)
	}
	return reasons
}

// hasUsableEvidence reports whether a memory carries provenance a reader can
// follow: at least one evidence entry with a trace ID that is not marked
// truncated-and-empty.
func hasUsableEvidence(m asymptoteobserve.LearningMemoryV1) bool {
	for _, e := range m.Evidence {
		if strings.TrimSpace(e.TraceID) != "" {
			return true
		}
	}
	return false
}

func isStale(m asymptoteobserve.LearningMemoryV1, now time.Time, maxAgeDays int) bool {
	ts := strings.TrimSpace(m.UpdatedAt)
	if ts == "" {
		ts = strings.TrimSpace(m.CreatedAt)
	}
	parsed, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return false
	}
	return now.Sub(parsed) > time.Duration(maxAgeDays)*24*time.Hour
}

// detectContradictions finds likely conflicts with deterministic rules: two
// memories in the same project and kind that share at least ContradictionTerms
// significant terms but express opposite polarity (one directive, one
// prohibition). It is a heuristic for human review, not a decision.
func detectContradictions(memories []asymptoteobserve.LearningMemoryV1, opts TrustOptions) []Contradiction {
	var out []Contradiction
	for i := 0; i < len(memories); i++ {
		for j := i + 1; j < len(memories); j++ {
			a, b := memories[i], memories[j]
			if a.Project.ID != b.Project.ID || a.Kind != b.Kind {
				continue
			}
			pa, pb := directivePolarity(a.Body), directivePolarity(b.Body)
			if pa == 0 || pb == 0 || pa == pb {
				continue
			}
			shared := sharedSignificantTerms(a.Body, b.Body, opts.ContradictionTerms)
			if len(shared) >= opts.ContradictionTerms {
				out = append(out, Contradiction{A: a, B: b, SharedTerm: shared})
			}
		}
	}
	return out
}

var (
	// Strong markers express an unambiguous obligation or prohibition.
	directiveStrongNegative = []string{"do not", "don't", "never", "must not", "should not", "shouldn't", "cannot"}
	directiveStrongPositive = []string{"always", "must", "ensure", "make sure", "required"}
	// Weak markers are advisory and are only consulted when no strong marker is
	// present, so "always cancel, avoid leaks" reads as a directive.
	directiveWeakNegative = []string{"avoid"}
	directiveWeakPositive = []string{"prefer", "should"}
)

// directivePolarity returns +1 for a directive, -1 for a prohibition, 0 for
// neither or for genuinely mixed text. Strong markers take precedence over weak
// ones so an advisory word inside a strong directive does not cancel it.
func directivePolarity(text string) int {
	lower := strings.ToLower(text)
	if p := polarityFrom(lower, directiveStrongNegative, directiveStrongPositive); p != 0 {
		return p
	}
	return polarityFrom(lower, directiveWeakNegative, directiveWeakPositive)
}

func polarityFrom(lower string, negatives, positives []string) int {
	neg, pos := false, false
	for _, marker := range negatives {
		if strings.Contains(lower, marker) {
			neg = true
		}
	}
	for _, marker := range positives {
		if strings.Contains(lower, marker) {
			pos = true
		}
	}
	switch {
	case neg && !pos:
		return -1
	case pos && !neg:
		return 1
	default:
		return 0
	}
}

var trustStopWords = map[string]bool{
	"the": true, "a": true, "an": true, "and": true, "or": true, "to": true,
	"of": true, "in": true, "on": true, "for": true, "is": true, "are": true,
	"be": true, "with": true, "this": true, "that": true, "it": true, "as": true,
	"do": true, "not": true, "never": true, "always": true, "must": true,
	"should": true, "avoid": true, "prefer": true, "use": true, "using": true,
}

func significantTerms(text string) map[string]bool {
	out := map[string]bool{}
	for _, w := range embedWords(strings.ToLower(text)) {
		if len(w) < 3 || trustStopWords[w] {
			continue
		}
		out[w] = true
	}
	return out
}

func sharedSignificantTerms(a, b string, min int) []string {
	ta, tb := significantTerms(a), significantTerms(b)
	var shared []string
	for w := range ta {
		if tb[w] {
			shared = append(shared, w)
		}
	}
	sort.Strings(shared)
	return shared
}
