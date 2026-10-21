package learning

import (
	"sync"
	"time"

	"github.com/asymptote-labs/agent-beacon/pkg/asymptoteobserve"
)

// search_cache.go is a bounded, process-local result cache for the adaptive
// search path. It is deliberately small: the smallest cache that produces a
// measurable warm-query win without adding a second source of truth.
//
// Correctness across processes rests on the durable `generation` counter in
// memory_index_meta, which PutMemory advances on every write. A cache entry is
// only served when the generation read from the database still matches the one
// the entry was stored under, so a write by any process invalidates the entry
// for every process on its next lookup. The check is one indexed single-row
// read, which is cheaper than the work it avoids and never returns a superseded
// or rescaled result.

const (
	// searchCacheMaxBytes bounds total cached result bytes. Entries are evicted
	// least-recently-used until the cache fits, so memory stays bounded no matter
	// how many distinct queries run.
	searchCacheMaxBytes = 32 << 20
	// searchCacheRankingVersion participates in the key so a change to ordering
	// or ranking can never reuse results produced by an older rule set.
	searchCacheRankingVersion = 1
)

type searchCacheKey struct {
	dbPath  string
	project string
	kind    string
	query   string
	limit   int
	page    int
	ranking int
}

type searchCacheEntry struct {
	generation string
	value      []asymptoteobserve.LearningMemoryV1
	size       int
	lastUsed   time.Time
}

var searchResultCache = struct {
	sync.Mutex
	entries map[searchCacheKey]*searchCacheEntry
	bytes   int
}{entries: make(map[searchCacheKey]*searchCacheEntry)}

func searchCacheKeyFor(dbPath string, query Query) searchCacheKey {
	return searchCacheKey{
		dbPath:  dbPath,
		project: query.ProjectID,
		kind:    query.Kind,
		query:   query.Q,
		limit:   normalizeLimit(query.Limit),
		page:    query.Page,
		ranking: searchCacheRankingVersion,
	}
}

func searchCacheEntrySize(value []asymptoteobserve.LearningMemoryV1) int {
	size := 64
	for _, m := range value {
		size += len(m.ID) + len(m.Title) + len(m.Body) + len(m.Kind) + len(m.UpdatedAt) + 128
	}
	return size
}

// searchCacheGet returns a cached result only when the generation passed in
// still matches the entry's. The caller reads the generation before it starts
// computing, so a write that commits during the computation makes the entry
// stale and it is never served.
func searchCacheGet(dbPath string, query Query, generation string) ([]asymptoteobserve.LearningMemoryV1, bool) {
	key := searchCacheKeyFor(dbPath, query)
	searchResultCache.Lock()
	defer searchResultCache.Unlock()
	entry, ok := searchResultCache.entries[key]
	if !ok || entry.generation != generation {
		return nil, false
	}
	entry.lastUsed = time.Now()
	return entry.value, true
}

func searchCachePut(dbPath string, query Query, generation string, value []asymptoteobserve.LearningMemoryV1) {
	key := searchCacheKeyFor(dbPath, query)
	entry := &searchCacheEntry{
		generation: generation,
		value:      value,
		size:       searchCacheEntrySize(value),
		lastUsed:   time.Now(),
	}
	if entry.size > searchCacheMaxBytes {
		return
	}
	searchResultCache.Lock()
	defer searchResultCache.Unlock()
	if existing, ok := searchResultCache.entries[key]; ok {
		searchResultCache.bytes -= existing.size
	}
	searchResultCache.entries[key] = entry
	searchResultCache.bytes += entry.size
	evictSearchCacheLocked()
}

func evictSearchCacheLocked() {
	for searchResultCache.bytes > searchCacheMaxBytes && len(searchResultCache.entries) > 0 {
		var oldestKey searchCacheKey
		var oldest *searchCacheEntry
		for key, entry := range searchResultCache.entries {
			if oldest == nil || entry.lastUsed.Before(oldest.lastUsed) {
				oldestKey, oldest = key, entry
			}
		}
		searchResultCache.bytes -= oldest.size
		delete(searchResultCache.entries, oldestKey)
	}
}

// resetSearchCacheForTest clears the process cache. Tests that share a database
// path across cases use it to avoid order dependence.
func resetSearchCacheForTest() {
	searchResultCache.Lock()
	defer searchResultCache.Unlock()
	searchResultCache.entries = make(map[searchCacheKey]*searchCacheEntry)
	searchResultCache.bytes = 0
}
