package learning

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"

	"github.com/asymptote-labs/agent-beacon/cli/beacon/internal/sqliteuri"
	"github.com/asymptote-labs/agent-beacon/pkg/asymptoteobserve"
)

const (
	storeFile          = "memory.db"
	storeSchemaVersion = 1
)

type Store struct {
	dbPath   string
	embedder Embedder
	// Set only by subprocess fault-injection tests.
	writeCheckpoint func(string)

	// mu guards lazy pool creation and Close. A Store owns exactly one *sql.DB
	// pool for its lifetime: opening it is deferred until the first operation
	// and reused by every subsequent operation, instead of opening and closing a
	// fresh pool per call.
	mu     sync.Mutex
	dbPool *sql.DB
	closed bool
}

// ErrStoreClosed is returned by store operations after Close.
var ErrStoreClosed = errors.New("learning: store is closed")

// faultHook, when non-nil, is called at named points during a memory write. It
// is a test seam for crash injection: it is nil in production, so each call site
// costs one nil comparison, and no fault-injection behaviour is reachable on a
// normal execution path. Tests set it to terminate the process at a chosen stage
// and then reopen the database to check durability.
var faultHook func(stage string)

// faultPoint invokes the test fault hook when one is installed.
func faultPoint(stage string) {
	if faultHook != nil {
		faultHook(stage)
	}
}

// Query selects stored learning records.
//
// ProjectPath is the trust boundary of this package. The CLI resolves an
// operator-supplied path itself and only ever sets ProjectID; ProjectPath is
// set solely by relayed callers such as the local dashboard and the MCP
// server, whose value arrives in a request. It is therefore matched against
// the projects the store already holds (Store.ProjectIDForPath) rather than
// resolved against the filesystem.
type Query struct {
	ProjectPath string
	ProjectID   string
	State       string
	Kind        string
	Q           string
	Limit       int
	Page        int
}

func Open(path string) *Store {
	if path == "" {
		path = storeFile
	}
	return &Store{dbPath: path}
}

func PathForRuntimeLog(logPath string) string {
	if strings.TrimSpace(logPath) == "" {
		return storeFile
	}
	dir := filepath.Dir(logPath)
	if filepath.Base(dir) == "logs" {
		return filepath.Join(filepath.Dir(dir), storeFile)
	}
	return filepath.Join(dir, storeFile)
}

func (s *Store) Path() string {
	return s.dbPath
}

// db returns the Store's shared, lazily created database pool. The pool is
// owned by the Store and released by Close; callers must not close it.
func (s *Store) db() (*sql.DB, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, ErrStoreClosed
	}
	if s.dbPool == nil {
		if err := s.openLocked(); err != nil {
			return nil, err
		}
	}
	return s.dbPool, nil
}

// openLocked creates the pool and initializes the schema. It must be called with
// s.mu held. On any failure the pool is closed so no handle leaks.
func (s *Store) openLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.dbPath), 0700); err != nil && filepath.Dir(s.dbPath) != "." {
		return err
	}
	// _txlock=immediate makes Begin take the write lock up front and wait on
	// busy_timeout. Thirty seconds bounds writer contention under heavy local
	// concurrency; a deferred transaction that reads before it writes cannot
	// wait for a competing writer without risking deadlock, so it fails with
	// SQLITE_BUSY instead; PutMemory's transaction must be immediate.
	db, err := sql.Open("sqlite", sqliteuri.File(s.dbPath, url.Values{
		"_pragma":       {"busy_timeout(30000)"},
		"_busy_timeout": {"30000"},
		"_txlock":       {"immediate"},
	}))
	if err != nil {
		return err
	}
	// Bound idle connections so an abandoned pool cannot hold file descriptors
	// open indefinitely; a Store still in use keeps reusing its pool.
	db.SetMaxIdleConns(2)
	db.SetConnMaxIdleTime(30 * time.Second)
	// busy_timeout is applied per connection by the DSN. journal_mode=WAL is
	// persistent in the database file and is set once, in ensureSchemaOnce, so a
	// read does not take a write lock just to open the store.
	if err := s.ensureSchemaOnce(db); err != nil {
		_ = db.Close()
		return err
	}
	s.dbPool = db
	return nil
}

// Close releases the Store's database pool. It is safe to call more than once;
// subsequent operations return ErrStoreClosed.
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	if s.dbPool != nil {
		err := s.dbPool.Close()
		s.dbPool = nil
		return err
	}
	return nil
}

func ensureSchema(db *sql.DB) error {
	var version int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		return err
	}
	if version > storeSchemaVersion {
		return fmt.Errorf("memory store schema %d is newer than this beacon supports (%d)", version, storeSchemaVersion)
	}
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS evaluations (
			id TEXT PRIMARY KEY,
			project_id TEXT NOT NULL,
			trace_id TEXT NOT NULL,
			status TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			rubric_hash TEXT NOT NULL,
			evaluator TEXT NOT NULL,
			evaluation_json TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS evaluations_by_project_updated ON evaluations (project_id, updated_at DESC)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS evaluations_by_project_trace_rubric ON evaluations (project_id, trace_id, rubric_hash)`,
		`CREATE TABLE IF NOT EXISTS candidates (
			id TEXT PRIMARY KEY,
			project_id TEXT NOT NULL,
			state TEXT NOT NULL,
			kind TEXT NOT NULL,
			title TEXT NOT NULL,
			source_evaluation_id TEXT,
			memory_id TEXT,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			candidate_json TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS candidates_by_project_state_updated ON candidates (project_id, state, updated_at DESC)`,
		`CREATE INDEX IF NOT EXISTS candidates_by_source_eval ON candidates (source_evaluation_id)`,
		`CREATE TABLE IF NOT EXISTS memories (
			id TEXT PRIMARY KEY,
			candidate_id TEXT NOT NULL,
			project_id TEXT NOT NULL,
			kind TEXT NOT NULL,
			title TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			superseded_by TEXT,
			memory_json TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS memories_by_project_updated ON memories (project_id, updated_at DESC)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS memories_by_candidate ON memories (candidate_id)`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	if version != storeSchemaVersion {
		if _, err := db.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, storeSchemaVersion)); err != nil {
			return err
		}
	}
	return nil
}

// ResolveProject reads git metadata to describe the project at path. It is for
// operator-supplied paths only: the CLI's --project flag and the process
// working directory. A path that arrives in a request is resolved by
// Store.ProjectIDForPath instead, which does not touch the filesystem.
func ResolveProject(path string) (asymptoteobserve.LearningProjectV1, error) {
	if strings.TrimSpace(path) == "" {
		var err error
		path, err = os.Getwd()
		if err != nil {
			return asymptoteobserve.LearningProjectV1{}, err
		}
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return asymptoteobserve.LearningProjectV1{}, err
	}
	root := findGitRoot(abs)
	if root == "" {
		root = abs
	}
	project := asymptoteobserve.LearningProjectV1{
		Path:      filepath.Clean(root),
		RemoteURL: readGitConfigValue(root, "remote \"origin\"", "url"),
		Branch:    readGitHeadBranch(root),
	}
	project.ID = ProjectID(project)
	return project, nil
}

// ProjectForTrace resolves the project a trace's memory belongs to. An explicit path
// wins. Otherwise the trace's own repository path is used when it recorded one, so
// memory lands with the repository the agent was working in rather than wherever the
// reviewer happened to run the command. With neither, it falls back to the working
// directory like ResolveProject.
func ProjectForTrace(path string, trace asymptoteobserve.TraceSummaryV1) (asymptoteobserve.LearningProjectV1, error) {
	if strings.TrimSpace(path) == "" && trace.Repository != nil {
		path = strings.TrimSpace(trace.Repository.Path)
	}
	return ResolveProject(path)
}

func ProjectID(project asymptoteobserve.LearningProjectV1) string {
	key := firstNonEmpty(project.RemoteURL, project.Path)
	if key == "" {
		key = "unknown"
	}
	sum := sha256.Sum256([]byte(strings.ToLower(filepath.ToSlash(key))))
	return "sha256:" + hex.EncodeToString(sum[:])
}

// KnownProjects returns the distinct projects this store holds records for.
func (s *Store) KnownProjects() ([]asymptoteobserve.LearningProjectV1, error) {
	db, err := s.db()
	if err != nil {
		return nil, err
	}
	rows, err := db.Query(`
		SELECT project_id, json_extract(evaluation_json, '$.project.path') FROM evaluations
		UNION
		SELECT project_id, json_extract(candidate_json, '$.project.path') FROM candidates
		UNION
		SELECT project_id, json_extract(memory_json, '$.project.path') FROM memories`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []asymptoteobserve.LearningProjectV1
	for rows.Next() {
		var id string
		var path sql.NullString
		if err := rows.Scan(&id, &path); err != nil {
			return nil, err
		}
		if strings.TrimSpace(id) == "" {
			continue
		}
		out = append(out, asymptoteobserve.LearningProjectV1{ID: id, Path: strings.TrimSpace(path.String)})
	}
	return out, rows.Err()
}

// ProjectIDForPath maps a project path supplied by a relayed caller onto the
// ID of a project this store already holds records for, choosing the most
// specific known project that contains the path. It deliberately never touches
// the filesystem: the dashboard and the MCP server pass a value that arrives in
// a request, and such a value must not reach a path expression. A path no
// stored project covers resolves to the same deterministic ID a plain
// directory would get, which matches no rows, so an unknown project keeps
// scoping the query to nothing rather than widening it to every project.
func (s *Store) ProjectIDForPath(path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", nil
	}
	cleaned, err := filepath.Abs(trimmed)
	if err != nil {
		return "", err
	}
	projects, err := s.KnownProjects()
	if err != nil {
		return "", err
	}
	var best asymptoteobserve.LearningProjectV1
	for _, project := range projects {
		if project.Path == "" || !pathWithin(cleaned, project.Path) {
			continue
		}
		if len(project.Path) > len(best.Path) {
			best = project
		}
	}
	if best.ID != "" {
		return best.ID, nil
	}
	return ProjectID(asymptoteobserve.LearningProjectV1{Path: cleaned}), nil
}

// scopeQuery resolves a relayed ProjectPath into a project ID once, before any
// statement runs. A caller that already knows the ID keeps it.
func (s *Store) scopeQuery(query Query) (Query, error) {
	if strings.TrimSpace(query.ProjectID) != "" || strings.TrimSpace(query.ProjectPath) == "" {
		query.ProjectPath = ""
		return query, nil
	}
	id, err := s.ProjectIDForPath(query.ProjectPath)
	if err != nil {
		return Query{}, err
	}
	query.ProjectID = id
	query.ProjectPath = ""
	return query, nil
}

// pathWithin reports whether path is root or sits under it. Comparison is
// case-insensitive to match how ProjectID folds case when it hashes a path.
func pathWithin(path, root string) bool {
	path = filepath.ToSlash(filepath.Clean(path))
	root = filepath.ToSlash(filepath.Clean(root))
	if strings.EqualFold(path, root) {
		return true
	}
	if !strings.HasSuffix(root, "/") {
		root += "/"
	}
	return strings.HasPrefix(strings.ToLower(path), strings.ToLower(root))
}

func findGitRoot(path string) string {
	current := filepath.Clean(path)
	info, err := os.Stat(current)
	if err == nil && !info.IsDir() {
		current = filepath.Dir(current)
	}
	for {
		if _, err := os.Stat(filepath.Join(current, ".git")); err == nil {
			return current
		}
		next := filepath.Dir(current)
		if next == current {
			return ""
		}
		current = next
	}
}

// gitDirs returns the git directory for root and the common directory that holds its
// config. In a linked worktree, .git is a file naming a gitdir under the main
// repository's .git/worktrees/<name>, and that gitdir's commondir file points back at
// the shared .git. Without following both, a session recorded in a worktree would hash
// to a path-based project instead of the remote its checkout shares.
func gitDirs(root string) (gitDir, commonDir string) {
	dotGit := filepath.Join(root, ".git")
	info, err := os.Stat(dotGit)
	if err != nil {
		return "", ""
	}
	if info.IsDir() {
		return dotGit, dotGit
	}
	data, err := os.ReadFile(dotGit)
	if err != nil {
		return "", ""
	}
	text := strings.TrimSpace(string(data))
	const prefix = "gitdir:"
	if !strings.HasPrefix(text, prefix) {
		return "", ""
	}
	gitDir = strings.TrimSpace(strings.TrimPrefix(text, prefix))
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(root, gitDir)
	}
	gitDir = filepath.Clean(gitDir)
	commonDir = gitDir
	if data, err := os.ReadFile(filepath.Join(gitDir, "commondir")); err == nil {
		common := strings.TrimSpace(string(data))
		if !filepath.IsAbs(common) {
			common = filepath.Join(gitDir, common)
		}
		commonDir = filepath.Clean(common)
	}
	return gitDir, commonDir
}

func readGitHeadBranch(root string) string {
	gitDir, _ := gitDirs(root)
	if gitDir == "" {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(gitDir, "HEAD"))
	if err != nil {
		return ""
	}
	text := strings.TrimSpace(string(data))
	const prefix = "ref: refs/heads/"
	if strings.HasPrefix(text, prefix) {
		return strings.TrimPrefix(text, prefix)
	}
	return ""
}

func readGitConfigValue(root, section, key string) string {
	_, commonDir := gitDirs(root)
	if commonDir == "" {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(commonDir, "config"))
	if err != nil {
		return ""
	}
	var inSection bool
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			inSection = strings.Trim(trimmed, "[]") == section
			continue
		}
		if !inSection || !strings.Contains(trimmed, "=") {
			continue
		}
		parts := strings.SplitN(trimmed, "=", 2)
		if strings.TrimSpace(parts[0]) == key {
			return strings.TrimSpace(parts[1])
		}
	}
	return ""
}

func (s *Store) Status() (asymptoteobserve.LearningStatusV1, error) {
	db, err := s.db()
	if err != nil {
		return asymptoteobserve.LearningStatusV1{}, err
	}
	status := asymptoteobserve.LearningStatusV1{
		SchemaVersion: asymptoteobserve.LearningSchemaVersion,
		Path:          s.dbPath,
		SizeBytes:     fileSize(s.dbPath),
	}
	_ = db.QueryRow(`SELECT COUNT(*) FROM evaluations`).Scan(&status.Evaluations)
	_ = db.QueryRow(`SELECT COUNT(*) FROM candidates`).Scan(&status.Candidates)
	_ = db.QueryRow(`SELECT COUNT(*) FROM memories WHERE superseded_by IS NULL OR superseded_by = ''`).Scan(&status.ApprovedMemories)
	return status, nil
}

func (s *Store) PutEvaluation(e asymptoteobserve.LearningEvaluationV1) error {
	if e.SchemaVersion == "" {
		e.SchemaVersion = asymptoteobserve.LearningSchemaVersion
	}
	now := nowString()
	if e.CreatedAt == "" {
		e.CreatedAt = now
	}
	e.UpdatedAt = now
	if e.ID == "" {
		e.ID = EvaluationID(e)
	}
	data, err := json.Marshal(e)
	if err != nil {
		return err
	}
	db, err := s.db()
	if err != nil {
		return err
	}
	_, err = db.Exec(`INSERT INTO evaluations (id, project_id, trace_id, status, created_at, updated_at, rubric_hash, evaluator, evaluation_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET status = excluded.status, updated_at = excluded.updated_at, evaluator = excluded.evaluator, evaluation_json = excluded.evaluation_json`,
		e.ID, e.Project.ID, e.Trace.ID, e.Status, e.CreatedAt, e.UpdatedAt, e.RubricHash, e.Evaluator, string(data))
	return err
}

func (s *Store) ListEvaluations(query Query) ([]asymptoteobserve.LearningEvaluationV1, error) {
	query, err := s.scopeQuery(query)
	if err != nil {
		return nil, err
	}
	db, err := s.db()
	if err != nil {
		return nil, err
	}
	where, args := queryWhere(query, "project_id", "status", "")
	rows, err := db.Query(`SELECT evaluation_json FROM evaluations`+where+` ORDER BY updated_at DESC LIMIT ? OFFSET ?`, append(args, normalizeLimit(query.Limit), offset(query))...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []asymptoteobserve.LearningEvaluationV1
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var value asymptoteobserve.LearningEvaluationV1
		if err := json.Unmarshal([]byte(raw), &value); err != nil {
			return nil, err
		}
		if query.Q == "" || matchesText(query.Q, value.ID, value.Trace.ID, value.Trace.Title, value.Evaluator, value.Status) {
			out = append(out, value)
		}
	}
	return out, rows.Err()
}

func (s *Store) GetEvaluation(id string) (asymptoteobserve.LearningEvaluationV1, bool, error) {
	var raw string
	ok, err := s.getJSON(`SELECT evaluation_json FROM evaluations WHERE id = ?`, id, &raw)
	if err != nil || !ok {
		return asymptoteobserve.LearningEvaluationV1{}, ok, err
	}
	var value asymptoteobserve.LearningEvaluationV1
	return value, true, json.Unmarshal([]byte(raw), &value)
}

func (s *Store) PutCandidate(c asymptoteobserve.LearningCandidateV1) error {
	if c.SchemaVersion == "" {
		c.SchemaVersion = asymptoteobserve.LearningSchemaVersion
	}
	now := nowString()
	if c.CreatedAt == "" {
		c.CreatedAt = now
	}
	c.UpdatedAt = now
	if c.State == "" {
		c.State = asymptoteobserve.LearningCandidateStateCandidate
	}
	if c.ID == "" {
		c.ID = CandidateID(c)
	}
	data, err := json.Marshal(c)
	if err != nil {
		return err
	}
	db, err := s.db()
	if err != nil {
		return err
	}
	_, err = db.Exec(`INSERT INTO candidates (id, project_id, state, kind, title, source_evaluation_id, memory_id, created_at, updated_at, candidate_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET state = excluded.state, kind = excluded.kind, title = excluded.title, memory_id = excluded.memory_id, updated_at = excluded.updated_at, candidate_json = excluded.candidate_json`,
		c.ID, c.Project.ID, c.State, c.Kind, c.Title, c.SourceEvaluationID, c.MemoryID, c.CreatedAt, c.UpdatedAt, string(data))
	return err
}

func (s *Store) ListCandidates(query Query) ([]asymptoteobserve.LearningCandidateV1, error) {
	query, err := s.scopeQuery(query)
	if err != nil {
		return nil, err
	}
	db, err := s.db()
	if err != nil {
		return nil, err
	}
	where, args := queryWhere(query, "project_id", "state", "kind")
	rows, err := db.Query(`SELECT candidate_json FROM candidates`+where+` ORDER BY updated_at DESC LIMIT ? OFFSET ?`, append(args, normalizeLimit(query.Limit), offset(query))...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []asymptoteobserve.LearningCandidateV1
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var value asymptoteobserve.LearningCandidateV1
		if err := json.Unmarshal([]byte(raw), &value); err != nil {
			return nil, err
		}
		if query.Q == "" || matchesText(query.Q, value.ID, value.Title, value.Body, value.Kind, value.State) {
			out = append(out, value)
		}
	}
	return out, rows.Err()
}

func (s *Store) GetCandidate(id string) (asymptoteobserve.LearningCandidateV1, bool, error) {
	var raw string
	ok, err := s.getJSON(`SELECT candidate_json FROM candidates WHERE id = ?`, id, &raw)
	if err != nil || !ok {
		return asymptoteobserve.LearningCandidateV1{}, ok, err
	}
	var value asymptoteobserve.LearningCandidateV1
	return value, true, json.Unmarshal([]byte(raw), &value)
}

func (s *Store) PutMemory(m asymptoteobserve.LearningMemoryV1) error {
	if m.SchemaVersion == "" {
		m.SchemaVersion = asymptoteobserve.LearningSchemaVersion
	}
	now := nowString()
	if m.CreatedAt == "" {
		m.CreatedAt = now
	}
	m.UpdatedAt = now
	if m.ID == "" {
		m.ID = MemoryID(m)
	}
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	db, err := s.db()
	if err != nil {
		return err
	}
	faultPoint("tx_begin_pre")
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin memory write: %w", err)
	}
	faultPoint("tx_begin_post")
	defer tx.Rollback()
	if s.writeCheckpoint != nil {
		s.writeCheckpoint("after-begin")
	}
	if _, err = tx.Exec(`INSERT INTO memories (id, candidate_id, project_id, kind, title, created_at, updated_at, superseded_by, memory_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET title = excluded.title, updated_at = excluded.updated_at, superseded_by = excluded.superseded_by, memory_json = excluded.memory_json`,
		m.ID, m.CandidateID, m.Project.ID, m.Kind, m.Title, m.CreatedAt, m.UpdatedAt, m.SupersededBy, string(data)); err != nil {
		return fmt.Errorf("insert memory: %w", err)
	}
	if s.writeCheckpoint != nil {
		s.writeCheckpoint("after-authoritative")
	}
	faultPoint("memory_insert")
	// Refresh the derived index row in the same transaction when the index is
	// known-good, and advance the durable generation either way so every process
	// sees that cached results for this store are now stale.
	indexHealthy := indexReadyTx(tx)
	if indexHealthy {
		if err := upsertMemoryIndex(tx, m); err != nil {
			// A derived-index problem must never fail the authoritative write.
			// Mark the index not-ready so the next search rebuilds it, and let
			// the authoritative row and the generation advance commit normally.
			indexHealthy = false
			if _, markErr := tx.Exec(`UPDATE memory_index_meta SET value = '0' WHERE key = 'index_ready'`); markErr != nil {
				return fmt.Errorf("index memory: %w", err)
			}
		}
	}
	if s.writeCheckpoint != nil {
		s.writeCheckpoint("after-index")
	}
	faultPoint("fts_update")
	if _, err := tx.Exec(`UPDATE memory_index_meta SET value = CAST(value AS INTEGER) + 1 WHERE key = 'generation'`); err != nil {
		return fmt.Errorf("bump generation: %w", err)
	}
	if s.writeCheckpoint != nil {
		s.writeCheckpoint("after-generation")
	}
	faultPoint("generation_update")
	if s.embedder != nil {
		// Embeddings are derived state, updated incrementally in the same
		// transaction. A failure here must never fail the authoritative write;
		// RebuildEmbeddings can repair it later.
		_ = s.upsertEmbeddingTx(tx, m)
	}
	if s.writeCheckpoint != nil {
		s.writeCheckpoint("before-commit")
	}
	faultPoint("pre_commit")
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit memory: %w", err)
	}
	if s.writeCheckpoint != nil {
		s.writeCheckpoint("after-commit")
	}
	faultPoint("post_commit")
	if !indexHealthy {
		// The derived index did not refresh with this write; mark it unhealthy
		// so the next search rebuilds it and re-ensure the derived schema in case
		// a table is missing. The authoritative row is already committed.
		s.markIndexUnhealthy(db)
	}
	return nil
}

// ListMemories returns approved (non-superseded) memories for a project. A query
// with no text uses the ordered, paged SQL path. A query with text dispatches to
// the adaptive search path (searchMemories), whose result is identical to the
// original scan this function used to run.
func (s *Store) ListMemories(query Query) ([]asymptoteobserve.LearningMemoryV1, error) {
	query, err := s.scopeQuery(query)
	if err != nil {
		return nil, err
	}
	db, err := s.db()
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(query.Q) == "" {
		return s.listMemoriesOrdered(db, query)
	}
	return s.searchMemories(db, query)
}

// listMemoriesOrdered is the original no-text path: project/kind scoped rows,
// newest first, limited and paged in SQL.
func (s *Store) listMemoriesOrdered(db *sql.DB, query Query) ([]asymptoteobserve.LearningMemoryV1, error) {
	where, args := queryWhere(query, "project_id", "", "kind")
	if where == "" {
		where = ` WHERE (superseded_by IS NULL OR superseded_by = '')`
	} else {
		where += ` AND (superseded_by IS NULL OR superseded_by = '')`
	}
	args = append(args, normalizeLimit(query.Limit), offset(query))
	rows, err := db.Query(`SELECT memory_json FROM memories`+where+` ORDER BY updated_at DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []asymptoteobserve.LearningMemoryV1
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var value asymptoteobserve.LearningMemoryV1
		if err := json.Unmarshal([]byte(raw), &value); err != nil {
			return nil, err
		}
		out = append(out, value)
	}
	return out, rows.Err()
}

// listMemoriesScan is the original text path and the correctness reference: it
// scans the scoped rows newest first, applies the in-Go matcher, and pages in
// Go. The indexed path reproduces exactly this result and falls back here.
func (s *Store) listMemoriesScan(db *sql.DB, query Query) ([]asymptoteobserve.LearningMemoryV1, error) {
	where, args := queryWhere(query, "project_id", "", "kind")
	if where == "" {
		where = ` WHERE (superseded_by IS NULL OR superseded_by = '')`
	} else {
		where += ` AND (superseded_by IS NULL OR superseded_by = '')`
	}
	rows, err := db.Query(`SELECT memory_json FROM memories`+where+` ORDER BY updated_at DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	limit := normalizeLimit(query.Limit)
	skip := offset(query)
	matches := 0
	var out []asymptoteobserve.LearningMemoryV1
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var value asymptoteobserve.LearningMemoryV1
		if err := json.Unmarshal([]byte(raw), &value); err != nil {
			return nil, err
		}
		if !matchesText(query.Q, value.ID, value.Title, value.Body, value.Kind) {
			continue
		}
		matches++
		if matches <= skip {
			continue
		}
		out = append(out, value)
		if len(out) >= limit {
			break
		}
	}
	return out, rows.Err()
}

func (s *Store) GetMemory(id string) (asymptoteobserve.LearningMemoryV1, bool, error) {
	var raw string
	ok, err := s.getJSON(`SELECT memory_json FROM memories WHERE id = ?`, id, &raw)
	if err != nil || !ok {
		return asymptoteobserve.LearningMemoryV1{}, ok, err
	}
	var value asymptoteobserve.LearningMemoryV1
	return value, true, json.Unmarshal([]byte(raw), &value)
}

func (s *Store) getJSON(query, id string, dest *string) (bool, error) {
	db, err := s.db()
	if err != nil {
		return false, err
	}
	err = db.QueryRow(query, id).Scan(dest)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}

func EvaluationID(e asymptoteobserve.LearningEvaluationV1) string {
	return idFor("eval", e.Project.ID, e.Trace.ID, e.RubricHash)
}

func CandidateID(c asymptoteobserve.LearningCandidateV1) string {
	var evidence []string
	for _, item := range c.Evidence {
		evidence = append(evidence, item.TraceID, strings.Join(item.EventIDs, ","))
	}
	return idFor("candidate", c.Project.ID, c.Kind, c.Title, c.Body, strings.Join(evidence, "|"))
}

func MemoryID(m asymptoteobserve.LearningMemoryV1) string {
	return idFor("memory", m.Project.ID, m.CandidateID, m.Kind, m.Title)
}

func idFor(prefix string, values ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(values, "\x00")))
	return prefix + "_" + hex.EncodeToString(sum[:16])
}

func queryWhere(query Query, projectColumn, stateColumn, kindColumn string) (string, []interface{}) {
	var clauses []string
	var args []interface{}
	projectID := strings.TrimSpace(query.ProjectID)
	if projectID != "" && projectColumn != "" {
		clauses = append(clauses, projectColumn+" = ?")
		args = append(args, projectID)
	}
	if query.State != "" && stateColumn != "" {
		clauses = append(clauses, stateColumn+" = ?")
		args = append(args, query.State)
	}
	if query.Kind != "" && kindColumn != "" {
		clauses = append(clauses, kindColumn+" = ?")
		args = append(args, query.Kind)
	}
	if len(clauses) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return 50
	}
	if limit > 500 {
		return 500
	}
	return limit
}

func offset(query Query) int {
	page := query.Page
	if page <= 1 {
		return 0
	}
	return (page - 1) * normalizeLimit(query.Limit)
}

func matchesText(q string, values ...string) bool {
	terms := strings.Fields(strings.ToLower(q))
	if len(terms) == 0 {
		return true
	}
	haystack := strings.ToLower(strings.Join(values, "\n"))
	for _, term := range terms {
		if !strings.Contains(haystack, term) {
			return false
		}
	}
	return true
}

func fileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

func nowString() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
