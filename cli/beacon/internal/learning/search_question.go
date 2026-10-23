package learning

import (
	"errors"
	"strings"

	"github.com/asymptote-labs/agent-beacon/pkg/asymptoteobserve"
)

// QuestionOptions controls the opt-in question-aware path. An embedder must be
// configured with OpenSemantic before semantic expansion is possible.
type QuestionOptions struct {
	CandidateK int
	MaxItems   int
	MaxTokens  int
	UseANN     bool
}

// QuestionResult contains the evidence given to an agent and the reason the
// route stopped. The caller must treat an insufficient result as uncertainty.
type QuestionResult struct {
	Context            ContextSelection    `json:"context"`
	Signals            TrustContextSignals `json:"signals"`
	Route              string              `json:"route"`
	EmbeddingModel     string              `json:"embedding_model,omitempty"`
	LexicalCandidates  int                 `json:"lexical_candidates"`
	SemanticCandidates int                 `json:"semantic_candidates"`
}

var errQuestionRequired = errors.New("question-aware search requires a question and scoped project")

// SearchQuestion starts with the original-compatible lexical path. It asks for
// semantic candidates only when the trusted lexical evidence does not directly
// support the full question. This path is opt-in; ListMemories is unchanged.
func (s *Store) SearchQuestion(query Query, question string, opts QuestionOptions) (QuestionResult, error) {
	var result QuestionResult
	query, err := s.scopeQuery(query)
	if err != nil {
		return result, err
	}
	if strings.TrimSpace(question) == "" || strings.TrimSpace(query.ProjectID) == "" || strings.TrimSpace(query.Q) == "" {
		return result, errQuestionRequired
	}
	if opts.CandidateK <= 0 {
		opts.CandidateK = 50
	}
	if opts.MaxItems <= 0 {
		opts.MaxItems = 8
	}
	query.Limit, query.Page = opts.CandidateK, 0
	lexical, err := s.ListMemories(query)
	if err != nil {
		return result, err
	}
	result.LexicalCandidates = len(lexical)
	result.Route = "lexical"
	trustOpts := TrustContextOptions{
		EnableSupersession: true, EnableTrust: true,
		EnableInjectionDefense: true,
	}
	trustedLexical := SelectTrustContext(lexical, question, trustOpts).Memories
	intent := analyzeQuestion(question, query.Q)
	var direct []asymptoteobserve.LearningMemoryV1
	var bridges []asymptoteobserve.LearningMemoryV1
	for _, m := range trustedLexical {
		switch {
		case intent.direct(m):
			direct = append(direct, m)
		case intent.primaryMatch(m):
			bridges = append(bridges, m)
		}
	}
	selected := direct
	if len(selected) == 0 && s.embedder != nil {
		result.Route = "semantic"
		result.EmbeddingModel = s.embedder.Model()
		db, err := s.db()
		if err != nil {
			return result, err
		}
		if err := s.ensureQuestionEmbeddings(db, query.ProjectID); err != nil {
			return result, err
		}
		qvec := s.embedder.Embed(query.Q)
		var hits []semanticHit
		if opts.UseANN {
			ids, err := s.semanticSearchANN(db, query, qvec, opts.CandidateK, defaultANNParams())
			if err != nil {
				return result, err
			}
			hits, err = s.memoriesByIDs(db, ids)
			if err != nil {
				return result, err
			}
		} else {
			hits, err = s.semanticSearchExact(db, query, qvec, opts.CandidateK)
			if err != nil {
				return result, err
			}
		}
		result.SemanticCandidates = len(hits)
		for _, hit := range hits {
			m := hit.memory
			if intent.direct(m) {
				selected = appendUniqueMemory(selected, m)
				continue
			}
			if !intent.slotMatch(m) || !intent.definitive(m) {
				continue
			}
			for _, bridge := range bridges {
				if intent.linked(bridge, m) {
					selected = appendUniqueMemory(selected, bridge)
					selected = appendUniqueMemory(selected, m)
					break
				}
			}
		}
	}

	// The final trust pass sees only question-relevant evidence. It cannot be
	// bypassed by lexical, semantic, ANN, or a cache hit in ListMemories.
	trustOpts.EnableConflict = true
	curated := SelectTrustContext(selected, question, trustOpts)
	result.Signals = curated.Signals
	result.Signals.Sufficient = len(curated.Memories) > 0 && !curated.Signals.Unresolved
	result.Signals.Insufficient = !result.Signals.Sufficient
	if result.Signals.Insufficient {
		if curated.Signals.Unresolved {
			result.Signals.SufficiencyNote = "relevant evidence contains an unresolved conflict"
		} else {
			result.Signals.SufficiencyNote = "no trusted evidence directly supports the question"
		}
	}
	result.Context = selectWholeQuestionContext(curated.Memories, opts.MaxItems, opts.MaxTokens)
	if len(result.Context.Items) == 0 {
		result.Signals.Sufficient, result.Signals.Insufficient = false, true
		result.Signals.SufficiencyNote = "no relevant evidence fits the context budget"
	} else if result.Route == "semantic" && result.Context.Dropped > 0 {
		result.Signals.Sufficient, result.Signals.Insufficient = false, true
		result.Signals.SufficiencyNote = "a needed evidence link does not fit the context budget"
	}
	return result, nil
}

// Whole memories keep the qualifiers needed to interpret a fact. If an item
// cannot fit, the caller sees insufficiency instead of a misleading fragment.
func selectWholeQuestionContext(memories []asymptoteobserve.LearningMemoryV1, maxItems, maxTokens int) ContextSelection {
	var result ContextSelection
	for _, m := range memories {
		cost := EstimateTokens(m.Title) + EstimateTokens(m.Body)
		result.TotalUnbounded += cost
		if (maxItems > 0 && len(result.Items) >= maxItems) || (maxTokens > 0 && result.TotalTokens+cost > maxTokens) {
			result.Dropped++
			continue
		}
		result.Items = append(result.Items, ContextItem{Memory: m, Tokens: cost})
		result.TotalTokens += cost
	}
	return result
}

func appendUniqueMemory(list []asymptoteobserve.LearningMemoryV1, m asymptoteobserve.LearningMemoryV1) []asymptoteobserve.LearningMemoryV1 {
	for _, existing := range list {
		if existing.ID == m.ID {
			return list
		}
	}
	return append(list, m)
}

type questionIntent struct {
	slot           string
	primary        string
	terms          map[string]bool
	allowUncertain bool
}

var questionNoise = map[string]bool{
	"which": true, "what": true, "where": true, "when": true, "how": true,
	"does": true, "should": true, "would": true, "could": true, "being": true,
	"added": true, "follow": true, "used": true, "uses": true, "use": true,
	"new": true, "now": true, "please": true, "tell": true, "about": true,
	"from": true, "into": true, "its": true, "this": true, "these": true,
	"listen": true, "listens": true, "must": true, "with": true,
	"first": true, "second": true, "third": true, "fourth": true,
	"currently": true, "applicable": true, "after": true, "before": true,
	"you": true, "your": true, "our": true, "ours": true,
}

var genericAnswerWords = map[string]bool{
	"value": true, "approach": true, "step": true, "action": true,
	"choice": true, "fixed": true, "fix": true, "answer": true,
	"information": true, "detail": true, "result": true,
}

func analyzeQuestion(question, retrievalQuery string) questionIntent {
	words := embedWords(strings.ToLower(question))
	intent := questionIntent{terms: map[string]bool{}}
	for _, word := range words {
		if uncertaintyWords[word] {
			intent.allowUncertain = true
			break
		}
	}
	queryWords := embedWords(strings.ToLower(retrievalQuery))
	for _, word := range queryWords {
		if len(word) >= 3 && !questionNoise[word] && !trustStopWords[word] {
			intent.primary = word
			break
		}
	}
	for i, word := range words {
		if word == "which" || word == "where" || word == "when" {
			for _, next := range words[i+1:] {
				if len(next) >= 3 && !questionNoise[next] && !trustStopWords[next] {
					intent.slot = next
					break
				}
			}
			if intent.slot != "" {
				break
			}
		} else if word == "what" || word == "how" {
			for _, next := range words[i+1:] {
				if len(next) >= 3 && !questionNoise[next] && !trustStopWords[next] {
					intent.slot = next
				}
			}
			break
		}
	}
	for _, word := range words {
		if len(word) < 3 || questionNoise[word] || trustStopWords[word] {
			continue
		}
		intent.terms[word] = true
	}
	if intent.slot == "" || genericAnswerWords[intent.slot] {
		// Generic request words describe the response format, not the fact.
		// The retrieval phrase supplies the concrete fact type in that case.
		for i := len(queryWords) - 1; i >= 0; i-- {
			if len(queryWords[i]) >= 3 && !questionNoise[queryWords[i]] && !trustStopWords[queryWords[i]] {
				intent.slot = queryWords[i]
				break
			}
		}
	}
	return intent
}

func (q questionIntent) memoryTerms(m asymptoteobserve.LearningMemoryV1) map[string]bool {
	return significantTerms(m.Title + " " + m.Body + " " + m.Applicability)
}

func (q questionIntent) slotMatch(m asymptoteobserve.LearningMemoryV1) bool {
	if q.slot == "" {
		return false
	}
	terms := q.memoryTerms(m)
	if coveredTerm(q.slot, terms) {
		return true
	}
	for _, alias := range slotAliases[q.slot] {
		if coveredTerm(alias, terms) {
			return true
		}
	}
	return false
}

func (q questionIntent) primaryMatch(m asymptoteobserve.LearningMemoryV1) bool {
	return q.primary != "" && coveredTerm(q.primary, q.memoryTerms(m))
}

func (q questionIntent) direct(m asymptoteobserve.LearningMemoryV1) bool {
	return q.slotMatch(m) && q.primaryMatch(m) && q.definitive(m)
}

func (q questionIntent) definitive(m asymptoteobserve.LearningMemoryV1) bool {
	if q.allowUncertain {
		return true
	}
	for _, word := range embedWords(strings.ToLower(m.Body)) {
		if uncertaintyWords[word] {
			return false
		}
	}
	return true
}

var uncertaintyWords = map[string]bool{
	"sometimes": true, "often": true, "usually": true, "typically": true,
	"might": true, "maybe": true, "possibly": true, "perhaps": true,
	"unconfirmed": true, "tentative": true, "unknown": true,
}

func (q questionIntent) linked(seed, candidate asymptoteobserve.LearningMemoryV1) bool {
	a, b := q.memoryTerms(seed), q.memoryTerms(candidate)
	for term := range a {
		if q.terms[term] || questionNoise[term] || bridgeNoise[term] {
			continue
		}
		if b[term] {
			return true
		}
	}
	return false
}

var slotAliases = map[string][]string{
	"environment": {"target", "destination"},
}

var bridgeNoise = map[string]bool{
	"service": true, "services": true, "team": true, "project": true,
	"data": true, "stores": true, "stored": true, "database": true,
}
