package learning

// Phase 4C: an opt-in, deterministic context-selection layer that runs after
// retrieval and before agent context assembly. It curates retrieved memories so
// the agent sees relevant, supported, non-obsolete evidence, and it preserves
// unresolved uncertainty instead of silently guessing.
//
// Design rules (from the Phase 4C brief):
//   - Do not blindly favour the newest memory (a newer note may be false).
//   - Do not blindly favour higher confidence (it may be manipulated).
//   - Prefer explicit, verifiable relationships: same subject/scope, compatible
//     project/version, supported correction/supersession, provenance, temporal
//     applicability.
//   - When a conflict cannot be resolved reliably, preserve the conflict and set
//     an uncertainty signal rather than choosing a possibly-wrong statement.
//
// Everything here is deterministic and rule-based; there is no model call. It is
// opt-in and changes no default behaviour.

import (
	"sort"
	"strings"
	"time"

	"github.com/asymptote-labs/agent-beacon/pkg/asymptoteobserve"
)

// TrustContextOptions configures SelectContext. Each mechanism is independently
// switchable so ablation experiments can isolate its effect.
type TrustContextOptions struct {
	Budget                 int  // max memories returned; <=0 means no limit
	EnableSupersession     bool // drop memories explicitly superseded/corrected
	EnableTrust            bool // drop untrusted-provenance memories
	EnableConflict         bool // detect conflicts; resolve or flag unresolved
	EnableInjectionDefense bool // detect + strip embedded instructions
	Now                    time.Time
}

// TrustContextSignals records what the layer did, for observability and evaluation.
type TrustContextSignals struct {
	SupersededIDs []string   `json:"superseded_ids"`
	ConflictPairs [][2]string `json:"conflict_pairs"`
	UntrustedIDs  []string   `json:"untrusted_ids"`
	InjectionIDs  []string   `json:"injection_ids"`
	SanitizedIDs  []string   `json:"sanitized_ids"`
	Unresolved    bool       `json:"unresolved"`
}

// TrustContext is the curated evidence plus the signals that produced it.
type TrustContext struct {
	Memories []asymptoteobserve.LearningMemoryV1 `json:"memories"`
	Signals  TrustContextSignals                      `json:"signals"`
}

// SelectContext curates retrieved memories. It never adds memories and never
// approves anything; it only selects, sanitizes, and annotates.
func SelectTrustContext(memories []asymptoteobserve.LearningMemoryV1, query string, opts TrustContextOptions) TrustContext {
	sig := TrustContextSignals{}
	kept := make([]asymptoteobserve.LearningMemoryV1, 0, len(memories))
	byID := map[string]asymptoteobserve.LearningMemoryV1{}
	for _, m := range memories {
		byID[m.ID] = m
	}

	// 1. Injection defense: sanitize or drop embedded instructions.
	sanitized := map[string]string{}
	dropInjection := map[string]bool{}
	for _, m := range memories {
		if hasInjection(m.Body) {
			sig.InjectionIDs = append(sig.InjectionIDs, m.ID)
			if opts.EnableInjectionDefense {
				clean := stripInstructions(m.Body)
				if strings.TrimSpace(clean) == "" {
					dropInjection[m.ID] = true // pure instruction -> drop
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

	// 3. Supersession: explicit correction relationships only.
	superseded := map[string]bool{}
	if opts.EnableSupersession {
		for _, m := range memories {
			if m.SupersededBy != "" {
				superseded[m.ID] = true
			}
		}
		for _, pair := range detectSupersessions(memories) {
			older, newer := pair[0], pair[1]
			// Only apply a correction from a memory that is not untrusted; a
			// low-trust note must not override verified knowledge.
			if opts.EnableTrust && untrusted[newer] {
				continue
			}
			if !untrusted[newer] {
				superseded[older] = true
			}
		}
		for id := range superseded {
			sig.SupersededIDs = append(sig.SupersededIDs, id)
		}
		sort.Strings(sig.SupersededIDs)
	}

	// 4. Conflict detection among survivors.
	conflicts := map[string]bool{}
	if opts.EnableConflict {
		remaining := make([]asymptoteobserve.LearningMemoryV1, 0, len(memories))
		for _, m := range memories {
			remaining = append(remaining, m)
		}
		for _, c := range detectContradictions(remaining, TrustOptions{AllowCrossKind: true, ContradictionTerms: 2}) {
			sig.ConflictPairs = append(sig.ConflictPairs, [2]string{c.A.ID, c.B.ID})
			// A conflict is resolved if one side is superseded or untrusted;
			// otherwise it is genuine and both sides are preserved.
			aDead := superseded[c.A.ID] || untrusted[c.A.ID]
			bDead := superseded[c.B.ID] || untrusted[c.B.ID]
			if aDead && !bDead {
				conflicts[c.A.ID] = true
			} else if bDead && !aDead {
				conflicts[c.B.ID] = true
			} else if !aDead && !bDead {
				sig.Unresolved = true
			}
		}
	}

	// 5. Assemble survivors.
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

	// 6. Budget: keep verified before untrusted, then by retrieval order.
	if opts.Budget > 0 && len(kept) > opts.Budget {
		sort.SliceStable(kept, func(i, j int) bool {
			return !isUntrustedSource(kept[i]) && isUntrustedSource(kept[j])
		})
		kept = kept[:opts.Budget]
	}

	sig.ConflictPairs = uniquePairs(sig.ConflictPairs)
	return TrustContext{Memories: kept, Signals: sig}
}

// ---- supersession ----

var correctionMarkers = []string{
	"now", "changed to", "changed from", "updated to", "raised to", "lowered to",
	"replaced", "no longer", "instead of", "corrected", "revised", "supersedes",
	"superseded", "migrated to", "switched to", "overridden", "deprecated",
}

// detectSupersessions returns ordered [older, newer] pairs where the newer
// memory explicitly corrects the older one about the same subject/scope.
func detectSupersessions(memories []asymptoteobserve.LearningMemoryV1) [][2]string {
	var out [][2]string
	for i := 0; i < len(memories); i++ {
		for j := 0; j < len(memories); j++ {
			if i == j {
				continue
			}
			older, newer := memories[i], memories[j]
			if older.Project.ID != newer.Project.ID {
				continue
			}
			if distinctExplicitVersions(older.Applicability, newer.Applicability) {
				continue // different versions -> both may be correct
			}
			if !hasCorrectionMarker(newer.Body) {
				continue
			}
			shared := intersectTerms(significantTerms(older.Body), significantTerms(newer.Body))
			if len(shared) < 2 {
				continue
			}
			// Require the shared subject terms to be at least half of the smaller
			// memory's significant terms. This rejects coincidental overlaps (e.g.
			// a "which service uses which database" note sharing only the database
			// name with a port correction) while accepting genuine same-subject
			// corrections.
			minTerms := min(len(significantTerms(older.Body)), len(significantTerms(newer.Body)))
			if minTerms > 0 && len(shared)*2 < minTerms {
				continue
			}
			if !isNewer(newer, older) {
				continue
			}
			out = append(out, [2]string{older.ID, newer.ID})
		}
	}
	return out
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
	"ignore all", "ignore the", "ignore other", "disregard", "forget the",
	"system:", "developer:", "assistant:", "you must", "you should now",
	"do not tell", "don't tell", "reveal", "expose the secret", "send the",
	"admin override", "override instruction", "important instruction",
	"new instruction", "ignore previous", "ignore above", "as an ai",
	"print the secret", "curl ", "rm -rf", "exfiltrate",
}

func hasInjection(body string) bool {
	lower := strings.ToLower(body)
	for _, p := range injectionPatterns {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}

// stripInstructions removes sentences that look like embedded instructions,
// keeping genuinely useful factual sentences (mixed malicious + useful content).
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
