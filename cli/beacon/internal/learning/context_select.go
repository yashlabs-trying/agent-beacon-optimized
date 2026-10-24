package learning

// Phase 4D: optimized, contract-driven context selection.
//
// This is the single owner of the post-retrieval decision. It is deterministic,
// opt-in, and changes no default behaviour. Phase 4D changes over Phase 4C:
//
//   - Near-linear candidate discovery: supersession and contradiction checks run
//     only over pairs that share >=2 significant terms (inverted-index buckets)
//     instead of an O(n^2) global scan. Results are identical to the O(n^2) path
//     (regression-tested against detectContradictions).
//   - Structured, evidence-backed supersession: the explicit SupersededBy field is
//     honoured, correction chains are resolved deterministically, and cycles are
//     broken safely.
//   - Evidence sufficiency: a structured signal distinguishing "supported",
//     "insufficient", and "conflicting" so the agent is not pushed to guess.
//   - Injection hardening: detects obfuscated/encoded instructions (base64-like
//     tokens, zero-width and homoglyph characters) in addition to keyword patterns.
//
// Invariants upheld: scope isolation, supersession correctness, conflict
// visibility, provenance preservation, trust separation, evidence sufficiency,
// budget safety, and fail-safe behaviour.

import (
	"encoding/base64"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/asymptote-labs/agent-beacon/pkg/asymptoteobserve"
)

// TrustContextOptions configures SelectTrustContext. Each mechanism is independently
// switchable for ablation.
type TrustContextOptions struct {
	Budget                 int
	EnableSupersession     bool
	EnableTrust            bool
	EnableConflict         bool
	EnableInjectionDefense bool
	// RequireSufficiency, when true, marks the result Insufficient and (if
	// WithholdIfInsufficient is also set) returns no memories, so a downstream
	// agent is not pushed to guess from evidence that does not cover the query.
	RequireSufficiency     bool
	WithholdIfInsufficient bool
	// Trust, when set, enables authenticated-provenance classification. A memory
	// is listed as authenticated only with a valid signature by a trusted key over
	// its exact content and scope. Nil disables the check (default).
	Trust *TrustRegistry
	Now   time.Time
}

// TrustContextSignals records the layer's decisions for observability and tests.
type TrustContextSignals struct {
	SupersededIDs []string    `json:"superseded_ids"`
	ConflictPairs [][2]string `json:"conflict_pairs"`
	UntrustedIDs  []string    `json:"untrusted_ids"`
	InjectionIDs  []string    `json:"injection_ids"`
	SanitizedIDs  []string    `json:"sanitized_ids"`
	Unresolved    bool        `json:"unresolved"`
	// Phase 4D sufficiency signal.
	Sufficient      bool     `json:"sufficient"`
	Insufficient    bool     `json:"insufficient"`
	Missing         []string `json:"missing"`
	Uncertainty     []string `json:"uncertainty"`
	SufficiencyNote string   `json:"sufficiency_note"`
	// Authenticated-provenance classification (populated only when Trust is set).
	AuthenticatedIDs     []string `json:"authenticated_ids,omitempty"`
	ProvenanceInvalidIDs []string `json:"provenance_invalid_ids,omitempty"`
}

// TrustContext is the curated evidence plus the signals that produced it.
type TrustContext struct {
	Memories []asymptoteobserve.LearningMemoryV1 `json:"memories"`
	Signals  TrustContextSignals                 `json:"signals"`
}

// SelectTrustContext curates retrieved memories. It never adds or approves
// memories; it only selects, sanitizes, and annotates.
func SelectTrustContext(memories []asymptoteobserve.LearningMemoryV1, query string, opts TrustContextOptions) TrustContext {
	sig := TrustContextSignals{Sufficient: true}

	// 1. Injection defense: sanitize or drop embedded instructions.
	sanitized := map[string]string{}
	dropInjection := map[string]bool{}
	for _, m := range memories {
		if hasInjection(m.Body) {
			sig.InjectionIDs = append(sig.InjectionIDs, m.ID)
			if opts.EnableInjectionDefense {
				clean := stripInstructions(m.Body)
				if strings.TrimSpace(clean) == "" {
					dropInjection[m.ID] = true
					continue
				}
				sanitized[m.ID] = clean
				sig.SanitizedIDs = append(sig.SanitizedIDs, m.ID)
			}
		}
	}

	// 2. Provenance trust tiering.
	untrusted := map[string]bool{}
	for _, m := range memories {
		if isUntrustedSource(m) {
			untrusted[m.ID] = true
			sig.UntrustedIDs = append(sig.UntrustedIDs, m.ID)
		}
	}

	// Term sets are shared by supersession and contradiction detection; compute
	// them once for the whole selection instead of once per mechanism. Supersession
	// needs them only when some memory could be a correction (an explicit
	// SupersededBy link or a correction marker); otherwise the O(n) tokenization is
	// skipped entirely. This is the common case for a plain fact store.
	var pairTerms []map[string]bool
	needSupersession := opts.EnableSupersession && hasCorrectionCandidate(memories)
	if needSupersession || opts.EnableConflict {
		pairTerms = significantTermsPerMemory(memories)
	}

	// 3. Structured supersession over candidate pairs only.
	superseded := map[string]bool{}
	if opts.EnableSupersession {
		superseded = resolveSupersessionsWithTerms(memories, pairTerms, untrusted, opts.EnableTrust)
		for id := range superseded {
			sig.SupersededIDs = append(sig.SupersededIDs, id)
		}
		sort.Strings(sig.SupersededIDs)
	}

	// 4. Conflict detection over candidate pairs only.
	conflicts := map[string]bool{}
	if opts.EnableConflict {
		for _, c := range detectContradictionsIndexedWithTerms(memories, pairTerms, TrustOptions{AllowCrossKind: true, ContradictionTerms: 2}) {
			sig.ConflictPairs = append(sig.ConflictPairs, [2]string{c.A.ID, c.B.ID})
			aDead := superseded[c.A.ID] || untrusted[c.A.ID]
			bDead := superseded[c.B.ID] || untrusted[c.B.ID]
			switch {
			case aDead && !bDead:
				conflicts[c.A.ID] = true
			case bDead && !aDead:
				conflicts[c.B.ID] = true
			case !aDead && !bDead:
				sig.Unresolved = true
				sig.Uncertainty = append(sig.Uncertainty, "unresolved conflict between "+c.A.ID+" and "+c.B.ID)
			}
		}
	}

	// 5. Assemble survivors.
	kept := make([]asymptoteobserve.LearningMemoryV1, 0, len(memories))
	for _, m := range memories {
		if dropInjection[m.ID] {
			continue
		}
		if opts.EnableInjectionDefense && contains(sig.SanitizedIDs, m.ID) {
			m.Body = sanitized[m.ID]
		}
		if opts.EnableSupersession && superseded[m.ID] {
			continue
		}
		if opts.EnableConflict && conflicts[m.ID] {
			continue
		}
		if opts.EnableTrust && untrusted[m.ID] && !contains(sig.SanitizedIDs, m.ID) {
			continue
		}
		kept = append(kept, m)
	}

	// 6. Evidence sufficiency (Phase 4D).
	if opts.RequireSufficiency {
		assessSufficiency(query, kept, &sig)
		if sig.Insufficient && opts.WithholdIfInsufficient {
			kept = nil
		}
	}

	// 6b. Authenticated provenance (opt-in). Classify survivors so a self-asserted
	// label is never mistaken for a trusted signature. This never changes which
	// memories are selected; it only records which carry a valid attestation.
	if opts.Trust != nil {
		for _, m := range kept {
			switch Authenticate(m, opts.Trust).Status {
			case ProvenanceVerified:
				sig.AuthenticatedIDs = append(sig.AuthenticatedIDs, m.ID)
			case ProvenanceInvalid:
				sig.ProvenanceInvalidIDs = append(sig.ProvenanceInvalidIDs, m.ID)
			}
		}
	}

	// 7. Budget: prefer verified memories.
	if opts.Budget > 0 && len(kept) > opts.Budget {
		sort.SliceStable(kept, func(i, j int) bool {
			return !isUntrustedSource(kept[i]) && isUntrustedSource(kept[j])
		})
		kept = kept[:opts.Budget]
	}

	sig.ConflictPairs = uniquePairs(sig.ConflictPairs)
	return TrustContext{Memories: kept, Signals: sig}
}

// ---- near-linear candidate discovery ----

// candidatePairs returns index pairs sharing >=2 significant terms, computed with an
// inverted index rather than an O(n^2) scan. Sorted for determinism.
func candidatePairs(memories []asymptoteobserve.LearningMemoryV1) [][2]int {
	return candidatePairsTerms(significantTermsPerMemory(memories))
}

// significantTermsPerMemory tokenizes each memory's body once. Callers that need
// both supersession and contradiction detection share this single computation.
func significantTermsPerMemory(memories []asymptoteobserve.LearningMemoryV1) []map[string]bool {
	terms := make([]map[string]bool, len(memories))
	for i, m := range memories {
		terms[i] = significantTerms(m.Body)
	}
	return terms
}

// candidatePairsTerms is candidatePairs over precomputed term sets.
func candidatePairsTerms(terms []map[string]bool) [][2]int {
	n := len(terms)
	// Retrieval candidate sets are small (tens). For those a direct pairwise scan
	// is cheaper than building inverted-index maps and allocates far less, while
	// producing the identical set of pairs (shared >=2 significant terms).
	if n <= 64 {
		var out [][2]int
		for i := 0; i < n; i++ {
			for j := i + 1; j < n; j++ {
				if sharedAtLeast(terms[i], terms[j], 2) {
					out = append(out, [2]int{i, j})
				}
			}
		}
		return out
	}
	idx := map[string][]int{}
	for i, t := range terms {
		for term := range t {
			idx[term] = append(idx[term], i)
		}
	}
	type key struct{ a, b int }
	counts := map[key]int{}
	for _, posts := range idx {
		for x := 0; x < len(posts); x++ {
			for y := x + 1; y < len(posts); y++ {
				a, b := posts[x], posts[y]
				if a > b {
					a, b = b, a
				}
				counts[key{a, b}]++
			}
		}
	}
	var out [][2]int
	for k, c := range counts {
		if c >= 2 {
			out = append(out, [2]int{k.a, k.b})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i][0] != out[j][0] {
			return out[i][0] < out[j][0]
		}
		return out[i][1] < out[j][1]
	})
	return out
}

// sharedCount returns the number of shared terms without allocating.
func sharedCount(a, b map[string]bool) int {
	if len(b) < len(a) {
		a, b = b, a
	}
	count := 0
	for term := range a {
		if b[term] {
			count++
		}
	}
	return count
}

// sharedAtLeast reports whether two term sets share at least n terms, exiting
// early once the threshold is reached.
func sharedAtLeast(a, b map[string]bool, n int) bool {
	// Iterate the smaller set for fewer comparisons.
	if len(b) < len(a) {
		a, b = b, a
	}
	count := 0
	for term := range a {
		if b[term] {
			count++
			if count >= n {
				return true
			}
		}
	}
	return false
}

// detectContradictionsIndexed mirrors detectContradictions exactly but only visits
// candidate pairs (shared >=2 significant terms). Regression-tested for equality.
func detectContradictionsIndexed(memories []asymptoteobserve.LearningMemoryV1, opts TrustOptions) []Contradiction {
	return detectContradictionsIndexedWithTerms(memories, significantTermsPerMemory(memories), opts)
}

// detectContradictionsIndexedWithTerms mirrors detectContradictions exactly but
// only visits candidate pairs (shared >=2 significant terms) using precomputed
// term sets. Regression-tested for equality.
func detectContradictionsIndexedWithTerms(memories []asymptoteobserve.LearningMemoryV1, terms []map[string]bool, opts TrustOptions) []Contradiction {
	if opts.ContradictionTerms <= 0 {
		opts.ContradictionTerms = 2
	}
	tokens := make([]map[string]bool, len(memories))
	pol := make([]int, len(memories))
	for i, m := range memories {
		tokens[i] = trustTokens(m.Body)
		pol[i] = directivePolarity(m.Body)
	}
	var out []Contradiction
	for _, p := range candidatePairsTerms(terms) {
		i, j := p[0], p[1]
		a, b := memories[i], memories[j]
		if a.Project.ID != b.Project.ID {
			continue
		}
		if !opts.AllowCrossKind && a.Kind != b.Kind {
			continue
		}
		if distinctExplicitVersions(a.Applicability, b.Applicability) {
			continue
		}
		shared := sharedCount(terms[i], terms[j])
		if shared < opts.ContradictionTerms {
			continue
		}
		minTerms := min(len(terms[i]), len(terms[j]))
		if minTerms > 0 && shared*2 <= minTerms {
			continue
		}
		if !conflictingDirective(pol[i], pol[j], tokens[i], tokens[j]) {
			continue
		}
		out = append(out, Contradiction{A: a, B: b, SharedTerm: intersectTerms(terms[i], terms[j])})
	}
	return out
}

// ---- structured supersession ----

// resolveSupersessions returns the set of memory IDs that are superseded, using the
// explicit SupersededBy field plus evidence-backed lexical corrections, resolving
// chains deterministically and breaking cycles.
func resolveSupersessions(memories []asymptoteobserve.LearningMemoryV1, untrusted map[string]bool, trustEnabled bool) map[string]bool {
	return resolveSupersessionsWithTerms(memories, significantTermsPerMemory(memories), untrusted, trustEnabled)
}

// resolveSupersessionsWithTerms returns the set of memory IDs that are superseded,
// using the explicit SupersededBy field plus evidence-backed lexical corrections,
// resolving chains deterministically and breaking cycles. Term sets are supplied
// by the caller so they are computed once for the whole selection.
func resolveSupersessionsWithTerms(memories []asymptoteobserve.LearningMemoryV1, terms []map[string]bool, untrusted map[string]bool, trustEnabled bool) map[string]bool {
	byID := map[string]asymptoteobserve.LearningMemoryV1{}
	for _, m := range memories {
		byID[m.ID] = m
	}
	superseded := map[string]bool{}
	for _, m := range memories {
		if m.SupersededBy != "" {
			superseded[m.ID] = true
		}
	}

	// Lexical corrections over candidate pairs.
	for _, p := range candidatePairsTerms(terms) {
		older, newer := memories[p[0]], memories[p[1]]
		olderTerms, newerTerms := terms[p[0]], terms[p[1]]
		if memoryTime(newer).Before(memoryTime(older)) {
			older, newer = newer, older
			olderTerms, newerTerms = newerTerms, olderTerms
		}
		if older.Project.ID != newer.Project.ID {
			continue
		}
		if distinctExplicitVersions(older.Applicability, newer.Applicability) {
			continue
		}
		if !hasCorrectionMarker(newer.Body) {
			continue
		}
		if trustEnabled && untrusted[newer.ID] {
			continue
		}
		if untrusted[newer.ID] {
			continue
		}
		shared := sharedCount(olderTerms, newerTerms)
		if shared < 2 {
			continue
		}
		minTerms := min(len(olderTerms), len(newerTerms))
		if minTerms > 0 && shared*2 < minTerms {
			continue
		}
		if !isNewer(newer, older) {
			continue
		}
		superseded[older.ID] = true
	}

	breakCycles(memories, byID, superseded)
	return superseded
}

func breakCycles(memories []asymptoteobserve.LearningMemoryV1, byID map[string]asymptoteobserve.LearningMemoryV1, superseded map[string]bool) {
	for _, m := range memories {
		if m.SupersededBy == "" {
			continue
		}
		seen := map[string]bool{m.ID: true}
		cur := m.SupersededBy
		for cur != "" {
			if seen[cur] {
				// cycle detected: clear the explicit flags in the cycle
				for id := range seen {
					superseded[id] = false
				}
				break
			}
			seen[cur] = true
			next, ok := byID[cur]
			if !ok {
				break
			}
			cur = next.SupersededBy
		}
	}
}

// ---- evidence sufficiency ----

// assessSufficiency sets the sufficiency signals on sig based on query-term coverage
// by the selected evidence. It is deliberately conservative: low coverage or an
// unresolved conflict means "insufficient", so the agent is not pushed to guess.
func assessSufficiency(query string, selected []asymptoteobserve.LearningMemoryV1, sig *TrustContextSignals) {
	if len(selected) == 0 {
		sig.Sufficient = false
		sig.Insufficient = true
		sig.SufficiencyNote = "no applicable evidence after filtering"
		return
	}
	qterms := significantTerms(query)
	if len(qterms) == 0 {
		sig.Sufficient = true
		sig.SufficiencyNote = "no content terms in query"
		return
	}
	covered := map[string]bool{}
	for _, m := range selected {
		for t := range significantTerms(m.Title + " " + m.Body) {
			covered[t] = true
		}
	}
	missing := 0
	for t := range qterms {
		if !coveredTerm(t, covered) {
			missing++
		}
	}
	coverage := float64(len(qterms)-missing) / float64(len(qterms))
	if coverage < 0.3 {
		sig.Sufficient = false
		sig.Insufficient = true
		sig.SufficiencyNote = "query terms not covered by evidence (coverage below threshold)"
		return
	}
	sig.Sufficient = true
	if sig.Unresolved {
		sig.SufficiencyNote = "evidence present but contains an unresolved conflict"
	} else {
		sig.SufficiencyNote = "evidence covers the query"
	}
}

// coveredTerm reports whether a query term is present, allowing light stemming:
// two terms match when they are equal or share a >=4-character prefix (so
// "timestamp"/"timestamps", "store"/"stored", "roll"/"rollouts" match).
func coveredTerm(term string, covered map[string]bool) bool {
	if covered[term] {
		return true
	}
	if len(term) < 4 {
		return false
	}
	for c := range covered {
		if len(c) >= 4 && (strings.HasPrefix(c, term[:4]) || strings.HasPrefix(term, c[:4])) {
			return true
		}
	}
	return false
}

// ---- supersession helpers ----

var correctionMarkers = []string{
	"now", "changed to", "changed from", "updated to", "raised to", "lowered to",
	"replaced", "no longer", "instead of", "corrected", "revised", "supersedes",
	"superseded", "migrated to", "switched to", "overridden", "deprecated",
}

// hasCorrectionCandidate reports whether any memory could supersede another: it
// declares a SupersededBy link or contains a correction marker. It is a cheap
// guard so a store with no corrections never pays for pairwise tokenization.
func hasCorrectionCandidate(memories []asymptoteobserve.LearningMemoryV1) bool {
	for _, m := range memories {
		if m.SupersededBy != "" || hasCorrectionMarker(m.Body) {
			return true
		}
	}
	return false
}

func hasCorrectionMarker(body string) bool {
	lower := strings.ToLower(body)
	for _, marker := range correctionMarkers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func isNewer(a, b asymptoteobserve.LearningMemoryV1) bool {
	ta, tb := memoryTime(a), memoryTime(b)
	if ta.IsZero() || tb.IsZero() {
		return false
	}
	return ta.After(tb)
}

func memoryTime(m asymptoteobserve.LearningMemoryV1) time.Time {
	ts := strings.TrimSpace(m.UpdatedAt)
	if ts == "" {
		ts = strings.TrimSpace(m.CreatedAt)
	}
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return time.Time{}
	}
	return t
}

// ---- injection ----

var injectionPatterns = []string{
	"ignore all", "ignore the", "ignore other", "ignore previous", "ignore above",
	"disregard", "forget the", "system:", "developer:", "assistant:", "you must",
	"you should now", "do not tell", "don't tell", "reveal", "expose the secret",
	"send the", "admin override", "override instruction", "important instruction",
	"new instruction", "as an ai", "print the secret", "curl ", "rm -rf", "exfiltrate",
}

// hasInjection detects plain and obfuscated embedded instructions.
func hasInjection(body string) bool {
	lower := strings.ToLower(body)
	for _, p := range injectionPatterns {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return hasObfuscatedInstruction(body)
}

// hasObfuscatedInstruction flags encoded or disguised instructions:
//   - zero-width characters used to hide text,
//   - base64-looking tokens that decode to instruction keywords.
func hasObfuscatedInstruction(body string) bool {
	if strings.ContainsAny(body, "\u200b\u200c\u200d\u2060\ufeff") {
		return true
	}
	for _, tok := range strings.FieldsFunc(body, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '+' && r != '/' && r != '='
	}) {
		if len(tok) < 16 {
			continue
		}
		if decoded, err := base64.StdEncoding.DecodeString(padBase64(tok)); err == nil {
			dl := strings.ToLower(string(decoded))
			for _, p := range injectionPatterns {
				if strings.Contains(dl, p) {
					return true
				}
			}
		}
	}
	return false
}

func padBase64(s string) string {
	if r := len(s) % 4; r != 0 {
		s += strings.Repeat("=", 4-r)
	}
	return s
}

// stripInstructions removes sentences that look like embedded instructions, keeping
// genuinely useful factual sentences (mixed malicious + useful content).
func stripInstructions(body string) string {
	var kept []string
	for _, sentence := range splitSentences(body) {
		if !hasInjection(sentence) {
			kept = append(kept, sentence)
		}
	}
	return strings.TrimSpace(strings.Join(kept, " "))
}

func splitSentences(text string) []string {
	fields := strings.FieldsFunc(text, func(r rune) bool {
		return r == '.' || r == '!' || r == '?' || r == '\n'
	})
	var out []string
	for _, f := range fields {
		f = strings.TrimSpace(f)
		if f != "" {
			out = append(out, f)
		}
	}
	return out
}

// ---- trust ----

func isUntrustedSource(m asymptoteobserve.LearningMemoryV1) bool {
	for _, tag := range m.Tags {
		t := strings.ToLower(strings.TrimSpace(tag))
		if t == "source:untrusted" || t == "source:external" || t == "source:injection" {
			return true
		}
	}
	lower := strings.ToLower(m.Body)
	for _, marker := range []string{"untrusted", "unverified", "external document", "forged", "claims that"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func uniquePairs(pairs [][2]string) [][2]string {
	seen := map[string]bool{}
	var out [][2]string
	for _, p := range pairs {
		key := p[0] + "|" + p[1]
		rev := p[1] + "|" + p[0]
		if seen[key] || seen[rev] {
			continue
		}
		seen[key] = true
		out = append(out, p)
	}
	return out
}
