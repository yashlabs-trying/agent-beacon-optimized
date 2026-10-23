package learning

import (
	"regexp"
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
	// AllowCrossKind lets the detector compare memories of different Kinds.
	// It is off by default: the Phase 1 labelled matrix deliberately treats a
	// difference in Kind as a scope boundary (no conflict), while the Phase 3
	// held-out set labels some cross-Kind pairs as conflicts. Enabling it is an
	// explicit, opt-in choice; the default keeps the Phase 1 contract intact.
	AllowCrossKind bool
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
	// Mirror AnnotateTrust's default so a caller that passes zero options cannot
	// accidentally make every opposite-polarity pair a contradiction.
	if opts.ContradictionTerms <= 0 {
		opts.ContradictionTerms = 2
	}
	// Precompute polarity, term sets and tokens once. The pairwise scan is
	// O(n^2), so recomputing them inside it is wasteful on a real corpus.
	type entry struct {
		memory asymptoteobserve.LearningMemoryV1
		pol    int
		terms  map[string]bool
		tokens map[string]bool
	}
	entries := make([]entry, len(memories))
	for i, m := range memories {
		entries[i] = entry{
			memory: m,
			pol:    directivePolarity(m.Body),
			terms:  significantTerms(m.Body),
			tokens: trustTokens(m.Body),
		}
	}

	var out []Contradiction
	for i := 0; i < len(entries); i++ {
		a := entries[i]
		for j := i + 1; j < len(entries); j++ {
			b := entries[j]
			if a.memory.Project.ID != b.memory.Project.ID {
				continue
			}
			if !opts.AllowCrossKind && a.memory.Kind != b.memory.Kind {
				continue
			}
			if distinctExplicitVersions(a.memory.Applicability, b.memory.Applicability) {
				continue
			}
			shared := intersectTerms(a.terms, b.terms)
			if len(shared) < opts.ContradictionTerms {
				continue
			}
			// Two memories only contradict if they are about the same subject.
			// Requiring the shared terms to be more than half of the smaller
			// memory's significant terms rejects coincidental collisions on
			// generic directive verbs ("pin", "before", "always") that share two
			// words but concern different things.
			minTerms := min(len(a.terms), len(b.terms))
			if minTerms > 0 && len(shared)*2 <= minTerms {
				continue
			}
			if !conflictingDirective(a.pol, b.pol, a.tokens, b.tokens) {
				continue
			}
			out = append(out, Contradiction{A: a.memory, B: b.memory, SharedTerm: shared})
		}
	}
	return out
}

// conflictingDirective reports whether two memories about the same subject
// express opposite directives. The caller has already required them to share
// enough significant terms to concern the same subject, so the three signals
// below are not applied to unrelated text:
//   - opposite non-zero polarity markers ("always" vs "never"/"do not");
//   - an opposing action verb pair ("stop"/"continue", "keep"/"remove"), which
//     catches directives that use no explicit polarity marker at all;
//   - a negation asymmetry where exactly one side carries a negation word and at
//     least one side is a directive.
func conflictingDirective(aPol, bPol int, aTokens, bTokens map[string]bool) bool {
	if aPol != 0 && bPol != 0 && aPol != bPol {
		return true
	}
	if hasOpposingAction(aTokens, bTokens) {
		return true
	}
	if (aPol != 0 || bPol != 0) && hasNegation(aTokens) != hasNegation(bTokens) {
		return true
	}
	return false
}

// trustTokens lowercases and tokenizes a body for the action and negation rules.
func trustTokens(text string) map[string]bool {
	out := map[string]bool{}
	for _, w := range embedWords(strings.ToLower(text)) {
		out[w] = true
	}
	return out
}

// negationWords are tokens that flip the sense of a directive. The list is
// token-based (not substring-based) so "not" cannot fire inside "note".
var negationWords = map[string]bool{
	"never": true, "not": true, "cannot": true, "avoid": true,
	"without": true, "prohibit": true, "prohibited": true, "forbid": true,
	"forbidden": true, "disallow": true, "refuse": true, "dont": true,
}

func hasNegation(tokens map[string]bool) bool {
	for w := range tokens {
		if negationWords[w] {
			return true
		}
	}
	return false
}

// opposingActionPairs lists verb forms whose members direct opposite behaviour
// on the same action. Each pair is an explicit set of surface forms so matching
// stays precise and never fires on an unrelated word that merely shares a prefix.
var opposingActionPairs = [][2][]string{
	{
		{"stop", "stops", "stopped", "stopping", "halt", "halts", "halted", "halting"},
		{"continue", "continues", "continued", "continuing", "proceed", "proceeds", "proceeded", "proceeding"},
	},
	{
		{"keep", "keeps", "kept", "keeping", "retain", "retains", "retained", "retaining", "preserve", "preserves", "preserved", "preserving"},
		{"remove", "removes", "removed", "removing", "delete", "deletes", "deleted", "deleting", "discard", "discards", "discarded", "discarding", "drop", "drops", "dropped", "dropping"},
	},
	{
		{"enable", "enables", "enabled", "enabling"},
		{"disable", "disables", "disabled", "disabling"},
	},
	{
		{"allow", "allows", "allowed", "allowing", "permit", "permits", "permitted", "permitting"},
		{"deny", "denies", "denied", "denying", "block", "blocks", "blocked", "blocking"},
	},
	{
		{"accept", "accepts", "accepted", "accepting", "approve", "approves", "approved", "approving"},
		{"reject", "rejects", "rejected", "rejecting", "refuse", "refuses", "refused", "refusing"},
	},
	{
		{"include", "includes", "included", "including"},
		{"exclude", "excludes", "excluded", "excluding", "omit", "omits", "omitted", "omitting"},
	},
}

func hasOpposingAction(aTokens, bTokens map[string]bool) bool {
	for _, pair := range opposingActionPairs {
		if (matchesAnyToken(aTokens, pair[0]) && matchesAnyToken(bTokens, pair[1])) ||
			(matchesAnyToken(aTokens, pair[1]) && matchesAnyToken(bTokens, pair[0])) {
			return true
		}
	}
	return false
}

func matchesAnyToken(tokens map[string]bool, forms []string) bool {
	for _, f := range forms {
		if tokens[f] {
			return true
		}
	}
	return false
}

var explicitVersionPattern = regexp.MustCompile(`(?i)\b(?:version|v)\s*([0-9]+(?:\.[0-9]+)*)\b`)

// Only suppress a conflict when both applicability strings name one distinct,
// unambiguous software version. Ranges and alternatives remain candidates for
// review because they may overlap.
func distinctExplicitVersions(a, b string) bool {
	version := func(value string) string {
		lower := strings.ToLower(value)
		if strings.Contains(lower, " or ") || strings.Contains(lower, " through ") || strings.ContainsAny(lower, "><+,") {
			return ""
		}
		matches := explicitVersionPattern.FindAllStringSubmatch(lower, -1)
		if len(matches) != 1 {
			return ""
		}
		out := matches[0][1]
		for strings.HasSuffix(out, ".0") {
			out = strings.TrimSuffix(out, ".0")
		}
		return out
	}
	av, bv := version(a), version(b)
	return av != "" && bv != "" && av != bv
}

// intersectTerms returns the shared terms of two term sets, sorted.
func intersectTerms(a, b map[string]bool) []string {
	var shared []string
	for w := range a {
		if b[w] {
			shared = append(shared, w)
		}
	}
	sort.Strings(shared)
	return shared
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
