package learning

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/asymptote-labs/agent-beacon/pkg/asymptoteobserve"
)

// This file adds an adaptive, indexed retrieval path for approved-memory search
// without changing the observable contract of Store.ListMemories.
//
// The authoritative records remain the rows of the `memories` table. The FTS
// table added here is derived state: it can be dropped and rebuilt from
// `memories` at any time, and every query that uses it applies the original
// in-Go matcher to the hydrated rows as a final exact check. When the index is
// missing, stale, unsupported or damaged, the engine falls back to the original
// scan, which remains the reference semantics.
//
// The index mirrors the tokenizer already used by the endpoint history store
// (see internal/endpoint/dashboard/history_store.go): FTS5 with the trigram
// tokenizer and case_sensitive 1 over text lowercased by strings.ToLower, the
// same fold the in-Go matcher applies. A quoted trigram term matches exactly
// the rows that contain it as a substring, for terms of three or more
// characters. Terms shorter than three characters cannot be indexed by a
// trigram tokenizer and are always served by the scan path.

const (
	// searchIndexVersion is the derived-index layout version. It is stored in
	// memory_index_meta and is independent of the authoritative store schema, so
	// adding or changing the index never requires migrating `memories`.
	searchIndexVersion = 1

	// searchCandidateCap bounds how many indexed candidates one query will
	// materialize before the planner prefers the scan path. A query whose
	// posting list is larger than this is not selective enough for the indexed
	// path to win, so the bounded original scan is used instead. It is also the
	// document-frequency threshold above which a trigram is recorded as common.
	searchCandidateCap = 256

	// searchRebuildBatch bounds the memory used while rebuilding the derived
	// index from the authoritative table.
	searchRebuildBatch = 5000
)

// metaTableQuerier is satisfied by both *sql.DB and *sql.Tx, so the small
// metadata reads below work on either.
type metaTableQuerier interface {
	QueryRow(query string, args ...any) *sql.Row
}

// memoryHaystack reproduces byte-for-byte the text the original in-Go matcher
// searched: matchesText lowercases the query and the "\n"-joined
// (ID, Title, Body, Kind) haystack. Keeping this identical is what lets the
// indexed path over-approximate the original predicate instead of redefining it.
func memoryHaystack(m asymptoteobserve.LearningMemoryV1) string {
	return strings.ToLower(strings.Join([]string{m.ID, m.Title, m.Body, m.Kind}, "\n"))
}

// memorySearchTerms splits a query the way matchesText does. The bool reports
// whether every term is long enough for the trigram index; when it is false the
// caller must use the scan path, because a one- or two-character term cannot be
// answered by the index without risking false negatives.
func memorySearchTerms(q string) ([]string, bool) {
	terms := strings.Fields(strings.ToLower(q))
	if len(terms) == 0 {
		return nil, false
	}
	for _, term := range terms {
		if utf8.RuneCountInString(term) < 3 {
			return terms, false
		}
	}
	return terms, true
}

// ftsMatch renders ANDed substring terms as an FTS5 MATCH expression. Each term
// is a quoted phrase so punctuation and FTS operators in user text are inert;
// embedded double quotes are doubled, which is FTS5's own escape.
func ftsMatch(terms []string) string {
	parts := make([]string, len(terms))
	for i, term := range terms {
		parts[i] = `"` + strings.ReplaceAll(term, `"`, `""`) + `"`
	}
	return strings.Join(parts, " AND ")
}

// schemaInitialized records, per store path, that this process has already
// verified the authoritative schema and created the derived index tables.
// Re-running the CREATE IF NOT EXISTS statements on every open is pure overhead
// and, under concurrent readers and writers, a source of write-lock contention
// on the schema. It is cleared when an index error is detected so a later open
// can recreate a dropped table.
var schemaInitialized sync.Map

// schemaInitMu serializes schema initialization across Stores in this process so
// two Stores on the same path cannot run the DDL concurrently or observe a
// partially initialized schema.
var schemaInitMu sync.Mutex

// ensureSchemaOnce verifies the authoritative schema and creates the derived
// index tables once per process per path. The derived schema is best-effort: if
// it cannot be created, the authoritative schema is still marked ready and
// search falls back to the scan path.
func (s *Store) ensureSchemaOnce(db *sql.DB) error {
	if _, ok := schemaInitialized.Load(s.dbPath); ok {
		return nil
	}
	// Serialize initialization across Stores in this process. Re-check under the
	// lock: the first winner marks the path initialized before releasing it, so
	// later callers return without re-running the DDL.
	schemaInitMu.Lock()
	defer schemaInitMu.Unlock()
	if _, ok := schemaInitialized.Load(s.dbPath); ok {
		return nil
	}
	if _, err := db.Exec(`PRAGMA journal_mode=WAL`); err != nil {
		return err
	}
	if err := ensureSchema(db); err != nil {
		return err
	}
	faultPoint("migration_post_schema")
	_ = ensureSearchSchema(db)
	schemaInitialized.Store(s.dbPath, struct{}{})
	return nil
}

// ensureSearchSchema creates the derived index tables. It never touches the
// authoritative tables, so a failure here degrades search to the scan path
// rather than breaking the store.
func ensureSearchSchema(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS memory_index_meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)`); err != nil {
		return err
	}
	for key, value := range map[string]string{
		"index_ready":               "0",
		"generation":                "0",
		"index_version":             fmt.Sprintf("%d", searchIndexVersion),
		"common_refresh_generation": "0",
	} {
		if _, err := db.Exec(`INSERT INTO memory_index_meta (key, value) VALUES (?, ?) ON CONFLICT(key) DO NOTHING`, key, value); err != nil {
			return err
		}
	}
	// A layout change invalidates any previously built index. Rebuild lazily on
	// the next search rather than here, so opening the store stays cheap.
	var version string
	if err := db.QueryRow(`SELECT value FROM memory_index_meta WHERE key = 'index_version'`).Scan(&version); err == nil {
		if version != fmt.Sprintf("%d", searchIndexVersion) {
			_, _ = db.Exec(`UPDATE memory_index_meta SET value = ? WHERE key = 'index_version'`, fmt.Sprintf("%d", searchIndexVersion))
			_, _ = db.Exec(`UPDATE memory_index_meta SET value = '0' WHERE key = 'index_ready'`)
		}
	}
	// Note whether the index table already existed. A freshly (re)created index
	// table is empty even though index_ready may still read 1 from a previous
	// build, for example after the table was dropped. In that case force a
	// rebuild rather than trusting an empty index and silently returning no
	// matches.
	var existed int
	_ = db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'memories_fts'`).Scan(&existed)
	if _, err := db.Exec(`CREATE VIRTUAL TABLE IF NOT EXISTS memories_fts USING fts5(memory_id UNINDEXED, hay, tokenize='trigram case_sensitive 1')`); err != nil {
		_, _ = db.Exec(`UPDATE memory_index_meta SET value = '0' WHERE key = 'index_ready'`)
		return err
	}
	if existed == 0 {
		_, _ = db.Exec(`UPDATE memory_index_meta SET value = '0' WHERE key = 'index_ready'`)
	}
	// The vocabulary is an optional planner aid: it exposes each trigram's
	// document frequency so the planner can reject a common term in O(log n)
	// instead of walking its posting list. Search works without it.
	_, _ = db.Exec(`CREATE VIRTUAL TABLE IF NOT EXISTS memories_vocab USING fts5vocab(memories_fts, 'row')`)
	// memory_trigram_df holds only the trigrams whose document frequency exceeds
	// the planner cap. It is a small plain table with a primary-key index, so a
	// per-query lookup is a b-tree descent rather than the full scan an
	// fts5vocab lookup performs. It is rebuilt with the index.
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS memory_trigram_df (term TEXT PRIMARY KEY, doc INTEGER NOT NULL)`); err != nil {
		return err
	}
	if err := ensureSemanticSchema(db); err != nil {
		return err
	}
	return nil
}

func metaValue(q metaTableQuerier, key string) (string, bool) {
	var value string
	if err := q.QueryRow(`SELECT value FROM memory_index_meta WHERE key = ?`, key).Scan(&value); err != nil {
		return "", false
	}
	return value, true
}

func indexReadyTx(tx *sql.Tx) bool {
	value, ok := metaValue(tx, "index_ready")
	return ok && value == "1"
}

func currentGeneration(db *sql.DB) (string, bool) {
	return metaValue(db, "generation")
}

// upsertMemoryIndex refreshes one memory's derived index row inside the caller's
// write transaction, so the index and the authoritative row commit together.
func upsertMemoryIndex(tx *sql.Tx, m asymptoteobserve.LearningMemoryV1) error {
	if _, err := tx.Exec(`DELETE FROM memories_fts WHERE memory_id = ?`, m.ID); err != nil {
		return err
	}
	_, err := tx.Exec(`INSERT INTO memories_fts (memory_id, hay) VALUES (?, ?)`, m.ID, memoryHaystack(m))
	return err
}

// rebuildMemoryIndexTx replaces the whole derived index from the authoritative
// table inside one write transaction. It is used for the first build and after a
// detected index failure. Batched so a large store does not need to be held in
// memory at once.
func rebuildMemoryIndexTx(tx *sql.Tx) error {
	if _, err := tx.Exec(`DELETE FROM memories_fts`); err != nil {
		return err
	}
	faultPoint("index_rebuild")
	insert, err := tx.Prepare(`INSERT INTO memories_fts (memory_id, hay) VALUES (?, ?)`)
	if err != nil {
		return err
	}
	defer insert.Close()
	var lastRowid int64 = -1
	for {
		rows, err := tx.Query(`SELECT rowid, id, memory_json FROM memories WHERE rowid > ? ORDER BY rowid LIMIT ?`, lastRowid, searchRebuildBatch)
		if err != nil {
			return err
		}
		type item struct {
			rowid int64
			id    string
			hay   string
		}
		var items []item
		for rows.Next() {
			var rowid int64
			var id, raw string
			if err := rows.Scan(&rowid, &id, &raw); err != nil {
				rows.Close()
				return err
			}
			var m asymptoteobserve.LearningMemoryV1
			if err := json.Unmarshal([]byte(raw), &m); err != nil {
				rows.Close()
				return err
			}
			items = append(items, item{rowid: rowid, id: id, hay: memoryHaystack(m)})
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		rows.Close()
		if len(items) == 0 {
			break
		}
		for _, it := range items {
			if _, err := insert.Exec(it.id, it.hay); err != nil {
				return err
			}
		}
		lastRowid = items[len(items)-1].rowid
	}
	return refreshCommonTrigramsTx(tx)
}

// refreshCommonTrigramsTx rebuilds the common-trigram table from the FTS
// vocabulary. It stores only trigrams whose document frequency exceeds the
// planner cap; a term is common exactly when every one of its trigrams appears
// here. The table is small (most trigrams are rare) and is refreshed whenever the
// index is rebuilt.
func refreshCommonTrigramsTx(tx *sql.Tx) error {
	if _, err := tx.Exec(`DELETE FROM memory_trigram_df`); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO memory_trigram_df (term, doc)
		SELECT term, doc FROM memories_vocab WHERE doc > ?`, searchCandidateCap); err != nil {
		return err
	}
	generation, _ := metaValue(tx, "generation")
	if generation == "" {
		generation = "0"
	}
	_, err := tx.Exec(`UPDATE memory_index_meta SET value = ? WHERE key = 'common_refresh_generation'`, generation)
	return err
}

// indexRebuildMu serializes in-process rebuilds so a burst of readers does not
// launch several concurrent rebuilds that would contend for the write lock.
var indexRebuildMu sync.Mutex

// indexReadyCache is retained only so existing tests can clear it; the readiness
// decision no longer reads it. Index health is always taken from the durable
// `index_ready` flag, so a process cannot conceal an index another process
// marked unhealthy.
var indexReadyCache sync.Map

// ensureIndexReady reports whether the derived index reflects the authoritative
// table, rebuilding it once if needed. The durable `index_ready` flag is the
// single source of truth, so the decision is correct across processes. A rebuild
// failure leaves index_ready at 0 so the caller falls back to the scan path.
func (s *Store) ensureIndexReady(db *sql.DB) bool {
	if ready, ok := metaValue(db, "index_ready"); ok && ready == "1" {
		return true
	}
	indexRebuildMu.Lock()
	defer indexRebuildMu.Unlock()
	if ready, ok := metaValue(db, "index_ready"); ok && ready == "1" {
		return true
	}
	tx, err := db.Begin()
	if err != nil {
		return false
	}
	defer tx.Rollback()
	if err := rebuildMemoryIndexTx(tx); err != nil {
		return false
	}
	if _, err := tx.Exec(`UPDATE memory_index_meta SET value = '1' WHERE key = 'index_ready'`); err != nil {
		return false
	}
	if err := tx.Commit(); err != nil {
		return false
	}
	return true
}

// markIndexUnhealthy forces the next search to rebuild the derived index. It is
// used when a query against the index fails, so a damaged index is repaired
// rather than trusted again.
func (s *Store) markIndexUnhealthy(db *sql.DB) {
	_, _ = db.Exec(`UPDATE memory_index_meta SET value = '0' WHERE key = 'index_ready'`)
	indexReadyCache.Delete(s.dbPath)
	// The Store reuses one pool, so a later call will not re-run schema
	// initialization on its own. Re-check the schema now, in case a derived
	// table is missing, so the next search can rebuild the index.
	schemaInitialized.Delete(s.dbPath)
	_ = s.ensureSchemaOnce(db)
}

// termTrigrams returns the distinct 3-rune substrings of a term. A trigram
// tokenizer indexes exactly these, so a row that contains the term contains all
// of them.
func termTrigrams(term string) []string {
	runes := []rune(term)
	if len(runes) < 3 {
		return nil
	}
	seen := make(map[string]struct{}, len(runes))
	out := make([]string, 0, len(runes))
	for i := 0; i+3 <= len(runes); i++ {
		tri := string(runes[i : i+3])
		if _, ok := seen[tri]; ok {
			continue
		}
		seen[tri] = struct{}{}
		out = append(out, tri)
	}
	return out
}

// queryIsSelective reports whether the ANDed query can match at most
// searchCandidateCap rows. A term is selective when at least one of its trigrams
// is absent from memory_trigram_df, meaning that trigram's document frequency is
// at or below the cap and the term cannot match more rows than that. The whole
// query is selective when any term is, because an AND is bounded by its most
// selective term. The bool is false when the table is unavailable, in which case
// the caller falls back to a bounded posting-list probe.
func queryIsSelective(db *sql.DB, terms []string) (bool, bool) {
	// Collect each term's trigrams and the unique set for one batched lookup.
	// The previous shape issued one SELECT per term; a long multi-term query
	// (e.g. 40 words) therefore paid 40 round trips before deciding to scan,
	// which dominated its latency. A single batched lookup returns the same
	// verdict in one (or a few, chunked) statements.
	perTerm := make([][]string, len(terms))
	unique := make([]string, 0, len(terms)*6)
	seen := make(map[string]struct{}, len(terms)*6)
	for i, term := range terms {
		trigrams := termTrigrams(term)
		if len(trigrams) == 0 {
			// A term too short to have a trigram cannot be judged by the
			// index; fall back to the bounded probe exactly as before.
			return false, false
		}
		perTerm[i] = trigrams
		for _, tri := range trigrams {
			if _, ok := seen[tri]; ok {
				continue
			}
			seen[tri] = struct{}{}
			unique = append(unique, tri)
		}
	}
	// Chunk the IN list to stay under SQLite's variable limit for very long
	// queries while still collapsing per-term round trips to per-chunk ones.
	present := make(map[string]struct{}, len(unique))
	const trigramChunk = 900
	for start := 0; start < len(unique); start += trigramChunk {
		end := start + trigramChunk
		if end > len(unique) {
			end = len(unique)
		}
		part := unique[start:end]
		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(part)), ",")
		args := make([]any, len(part))
		for i, tri := range part {
			args[i] = tri
		}
		rows, err := db.Query(`SELECT term FROM memory_trigram_df WHERE term IN (`+placeholders+`)`, args...)
		if err != nil {
			return false, false
		}
		for rows.Next() {
			var term string
			if err := rows.Scan(&term); err != nil {
				rows.Close()
				return false, false
			}
			present[term] = struct{}{}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return false, false
		}
		rows.Close()
	}
	// A term is selective when at least one of its trigrams is absent from the
	// common-trigram table; the whole AND is selective if any term is.
	for _, trigrams := range perTerm {
		allPresent := true
		for _, tri := range trigrams {
			if _, ok := present[tri]; !ok {
				allPresent = false
				break
			}
		}
		if !allPresent {
			return true, true
		}
	}
	return false, true
}

// fetchIndexedCandidates runs the scoped indexed lookup for a text query that
// the planner has already judged selective. It returns (nil, nil) if the scoped
// posting list overflows the cap, which should not happen after a selective
// verdict but is handled by the caller as a fall back to the scan.
func (s *Store) fetchIndexedCandidates(db *sql.DB, query Query, terms []string) ([]asymptoteobserve.LearningMemoryV1, error) {
	var clauses []string
	var args []any
	args = append(args, ftsMatch(terms))
	if projectID := strings.TrimSpace(query.ProjectID); projectID != "" {
		clauses = append(clauses, `m.project_id = ?`)
		args = append(args, projectID)
	}
	if kind := strings.TrimSpace(query.Kind); kind != "" {
		clauses = append(clauses, `m.kind = ?`)
		args = append(args, kind)
	}
	clauses = append(clauses, `(m.superseded_by IS NULL OR m.superseded_by = '')`)
	args = append(args, searchCandidateCap+1)

	stmt := `SELECT m.memory_json
		FROM memories_fts
		JOIN memories m ON m.id = memories_fts.memory_id
		WHERE memories_fts MATCH ? AND ` + strings.Join(clauses, ` AND `) + `
		LIMIT ?`
	rows, err := db.QueryContext(query.context(), stmt, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]asymptoteobserve.LearningMemoryV1, 0, 16)
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		if len(out) == searchCandidateCap {
			// Defensive: the scoped fetch should never exceed the estimate.
			return nil, nil
		}
		var m asymptoteobserve.LearningMemoryV1
		if err := json.Unmarshal([]byte(raw), &m); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// probePostingListOverflow is the fallback selectivity check used only when the
// vocabulary is unavailable. It reads at most searchCandidateCap+1 index rows,
// with no join, so LIMIT can stop early.
func (s *Store) probePostingListOverflow(db *sql.DB, terms []string) (bool, error) {
	rows, err := db.Query(`SELECT memory_id FROM memories_fts WHERE memories_fts MATCH ? LIMIT ?`, ftsMatch(terms), searchCandidateCap+1)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		count++
		if count > searchCandidateCap {
			return true, nil
		}
	}
	return false, rows.Err()
}

// searchMemories is the adaptive path for a query that carries text. It selects
// between the indexed lookup and the original scan, then applies the original
// matcher as a final exact check before ordering and paginating. The final
// result is therefore identical to the scan path whenever the indexed path is
// used, and falls back to it otherwise.
func (s *Store) searchMemories(db *sql.DB, query Query) ([]asymptoteobserve.LearningMemoryV1, error) {
	if err := query.contextErr(); err != nil {
		return nil, err
	}
	terms, indexable := memorySearchTerms(query.Q)
	if !indexable || !s.ensureIndexReady(db) {
		// Short terms and an unavailable index go straight to the scan, so a
		// common query pays no extra reads beyond the original path.
		return s.listMemoriesScan(db, query)
	}
	selective, ok := queryIsSelective(db, terms)
	if !ok {
		overflow, err := s.probePostingListOverflow(db, terms)
		if err != nil {
			s.markIndexUnhealthy(db)
			return s.listMemoriesScan(db, query)
		}
		selective = !overflow
	}
	if !selective {
		return s.listMemoriesScan(db, query)
	}

	// Read the generation only on the indexed path. If a write commits while the
	// result is computed, it is stored under this older generation and the next
	// lookup misses, so a stale result is never served.
	generation, generationOK := currentGeneration(db)
	if generationOK {
		if cached, ok := searchCacheGet(s.dbPath, query, generation); ok {
			return cached, nil
		}
	}
	candidates, err := s.fetchIndexedCandidates(db, query, terms)
	if err != nil {
		// The derived index answered with an error: stop trusting it, fall back
		// to the authoritative scan, and let the next call rebuild.
		s.markIndexUnhealthy(db)
		return s.listMemoriesScan(db, query)
	}
	if candidates == nil {
		return s.listMemoriesScan(db, query)
	}
	out := make([]asymptoteobserve.LearningMemoryV1, 0, len(candidates))
	for _, m := range candidates {
		if matchesText(query.Q, m.ID, m.Title, m.Body, m.Kind) {
			out = append(out, m)
		}
	}
	sortMemories(out)
	out = paginateMemories(out, query)
	if generationOK {
		searchCachePut(s.dbPath, query, generation, out)
	}
	return out, nil
}

// sortMemories orders results newest-first, matching ORDER BY updated_at DESC.
// The ID tie-break makes equal timestamps deterministic, which the original
// scan left to SQLite's row order.
func sortMemories(list []asymptoteobserve.LearningMemoryV1) {
	sort.SliceStable(list, func(i, j int) bool {
		if list[i].UpdatedAt != list[j].UpdatedAt {
			return list[i].UpdatedAt > list[j].UpdatedAt
		}
		return list[i].ID < list[j].ID
	})
}

func paginateMemories(list []asymptoteobserve.LearningMemoryV1, query Query) []asymptoteobserve.LearningMemoryV1 {
	skip := offset(query)
	if skip >= len(list) {
		return nil
	}
	list = list[skip:]
	if limit := normalizeLimit(query.Limit); len(list) > limit {
		list = list[:limit]
	}
	return list
}
