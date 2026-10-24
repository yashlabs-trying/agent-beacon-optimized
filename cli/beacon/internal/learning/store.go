package learning

import (
	"context"
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
	"sync/atomic"
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

	// authorizer, when set, gates lifecycle mutations by the candidate's
	// project. It is configured by the entry point (CLI operator or MCP server);
	// nil means trusted embedding (direct library use by the store owner).
	authorizer ProjectAuthorizer

	// signingAuthority, when set, attests approved memory content with an
	// Ed25519 signature. trustRegistry holds the public keys trusted for
	// verification.
	signingAuthority *SigningAuthority
	trustRegistry    *TrustRegistry

	// mu guards lazy pool creation and Close. A Store owns exactly one *sql.DB
	// pool for its lifetime: opening it is deferred until the first operation
	// and reused by every subsequent operation, instead of opening and closing a
	// fresh pool per call.
	mu     sync.Mutex
	dbPool *sql.DB
	closed bool

	// writeMu serializes in-process writers so only one goroutine holds a
	// SQLite write transaction at a time. SQLite permits a single writer; with
	// _txlock=immediate, concurrent writers otherwise busy-poll the lock and
	// produce a long tail under 16+ client load. Serializing on a Go mutex makes
	// waiting goroutines park cheaply instead of spinning in the busy handler.
	// Durability is unchanged: every write still commits under WAL.
	writeMu sync.Mutex

	// busyRetries and busyExhausted count bounded cross-process BUSY retries and
	// the retries that gave up.
	busyRetries   atomic.Int64
	busyExhausted atomic.Int64
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

	// ctx, when set by ListMemoriesContext, is honoured by the query path so a
	// canceled or expired request stops promptly. It is unexported so a caller
	// cannot smuggle a context through a public struct literal.
	ctx context.Context
}

func (q Query) context() context.Context {
	if q.ctx != nil {
		return q.ctx
	}
	return context.Background()
}

func (q Query) contextErr() error {
	if q.ctx == nil {
		return nil
	}
	return q.ctx.Err()
}

func Open(path string) *Store {
	if path == "" {
		path = storeFile
	}
	return &Store{dbPath: path}
}

// OpenAuthorized opens a store whose lifecycle mutations are gated by a. An
// entry point that accepts callers who are not the store owner MUST use this
// (or SetAuthorizer) so mutations cannot cross a project boundary.
func OpenAuthorized(path string, a ProjectAuthorizer) *Store {
	store := Open(path)
	store.authorizer = a
	return store
}

// SetAuthorizer configures the lifecycle authorizer. Call it before the store is
// used concurrently. Passing nil selects trusted embedding.
func (s *Store) SetAuthorizer(a ProjectAuthorizer) { s.authorizer = a }

// SetSigningAuthority configures the approval signing key. When set, approved
// memories carry an Ed25519 attestation. Call it before the store is used.
func (s *Store) SetSigningAuthority(a *SigningAuthority) { s.signingAuthority = a }

// SetTrustRegistry configures the public keys trusted for provenance
// verification.
func (s *Store) SetTrustRegistry(r *TrustRegistry) { s.trustRegistry = r }

// AuthenticateMemory reports the provenance status of a memory against the
// store's trust registry. With no registry configured every record is
// Unverified (fail-safe).
func (s *Store) AuthenticateMemory(m asymptoteobserve.LearningMemoryV1) Provenance {
	return Authenticate(m, s.trustRegistry)
}

// authorizeLifecycle enforces the configured authorizer for a project, or allows
// the operation in trusted-embedding mode (no authorizer).
func (s *Store) authorizeLifecycle(projectID string) error {
	if s.authorizer == nil {
		return nil
	}
	return s.authorizer.AuthorizeProject(context.Background(), projectID)
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
	db.SetMaxOpenConns(8)
	db.SetConnMaxIdleTime(30 * time.Second)
	// busy_timeout is applied per connection by the DSN. journal_mode=WAL is
	// persistent in the database file and is set once, in ensureSchemaOnce, so a
	// read does not take a write lock just to open the store.
	//
	// Several processes opening the same fresh database race to create the
	// schema. Retry the initialization on a transient BUSY so a concurrent
	// first-open cannot fail a writer.
	var schemaErr error
	for attempt := 0; attempt < busyRetryMaxAttempts; attempt++ {
		schemaErr = s.ensureSchemaOnce(db)
		if schemaErr == nil || !isBusyError(schemaErr) {
			break
		}
		time.Sleep(time.Duration(attempt+1) * busyRetryBaseBackoff)
	}
	if schemaErr != nil {
		_ = db.Close()
		return schemaErr
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
		`CREATE TABLE IF NOT EXISTS audit (
			seq INTEGER PRIMARY KEY AUTOINCREMENT,
			ts TEXT NOT NULL,
			event TEXT NOT NULL,
			actor TEXT,
			project TEXT,
			target TEXT,
			outcome TEXT,
			request_id TEXT,
			detail TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS audit_by_project_ts ON audit (project, ts)`,
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
	c = normalizeCandidateForWrite(c)
	db, err := s.db()
	if err != nil {
		return err
	}
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin candidate write: %w", err)
	}
	defer tx.Rollback()
	if err := putCandidateTx(tx, c); err != nil {
		return err
	}
	return tx.Commit()
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
	if err := s.authorizeLifecycle(m.Project.ID); err != nil {
		return err
	}
	m = normalizeMemoryForWrite(m)
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	db, err := s.db()
	if err != nil {
		return err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return s.busyRetry(func() error {
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
		indexHealthy, err := s.writeMemoryTx(tx, m, data)
		if err != nil {
			return err
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
	})
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
	if err := query.contextErr(); err != nil {
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

// ListMemoriesContext is ListMemories that honours cancellation and deadlines.
// A canceled or expired context stops the scan promptly and returns the context
// error; no partial result is returned as if complete.
func (s *Store) ListMemoriesContext(ctx context.Context, query Query) ([]asymptoteobserve.LearningMemoryV1, error) {
	query.ctx = ctx
	return s.ListMemories(query)
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
	rows, err := db.QueryContext(query.context(), `SELECT memory_json FROM memories`+where+` ORDER BY updated_at DESC LIMIT ? OFFSET ?`, args...)
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

// searchScanRowCap bounds how many rows the in-Go scan decodes for a text query.
// Selective queries are answered by the bounded FTS5 path; only non-selective
// queries (short or common terms) reach the scan, which otherwise reads and
// JSON-decodes the whole project. The cap is generous for legitimate projects and
// turns a pathological scan into an explicit overload error instead of unbounded
// work. It is deliberately not a silent truncation: the caller sees an error.
// It is a var only so tests can lower it.
var searchScanRowCap = 200000

// ErrSearchScanExceeded is returned when a text-query scan exceeds
// searchScanRowCap. The result is unknown, not empty: callers must not present
// it as a complete answer.
var ErrSearchScanExceeded = errors.New("learning: text search scan exceeded the row cap; narrow the query")

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
	rows, err := db.QueryContext(query.context(), `SELECT memory_json FROM memories`+where+` ORDER BY updated_at DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	limit := normalizeLimit(query.Limit)
	skip := offset(query)
	matches := 0
	scanned := 0
	var out []asymptoteobserve.LearningMemoryV1
	for rows.Next() {
		if err := query.contextErr(); err != nil {
			return nil, err
		}
		scanned++
		if scanned > searchScanRowCap {
			return nil, ErrSearchScanExceeded
		}
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

// GetMemoryContext is GetMemory that honours cancellation and deadlines.
func (s *Store) GetMemoryContext(ctx context.Context, id string) (asymptoteobserve.LearningMemoryV1, bool, error) {
	db, err := s.db()
	if err != nil {
		return asymptoteobserve.LearningMemoryV1{}, false, err
	}
	var raw string
	err = db.QueryRowContext(ctx, `SELECT memory_json FROM memories WHERE id = ?`, id).Scan(&raw)
	if err == sql.ErrNoRows {
		return asymptoteobserve.LearningMemoryV1{}, false, nil
	}
	if err != nil {
		return asymptoteobserve.LearningMemoryV1{}, false, err
	}
	var value asymptoteobserve.LearningMemoryV1
	return value, true, json.Unmarshal([]byte(raw), &value)
}

// GetMemoryInProject is GetMemory with the project scope pushed into the query,
// so a caller can never fetch a row outside the project it is authorized for.
func (s *Store) GetMemoryInProject(id, projectID string) (asymptoteobserve.LearningMemoryV1, bool, error) {
	return s.GetMemoryInProjectContext(context.Background(), id, projectID)
}

// GetMemoryInProjectContext is GetMemoryInProject that honours cancellation.
func (s *Store) GetMemoryInProjectContext(ctx context.Context, id, projectID string) (asymptoteobserve.LearningMemoryV1, bool, error) {
	db, err := s.db()
	if err != nil {
		return asymptoteobserve.LearningMemoryV1{}, false, err
	}
	var raw string
	err = db.QueryRowContext(ctx, `SELECT memory_json FROM memories WHERE id = ? AND project_id = ?`, id, projectID).Scan(&raw)
	if err == sql.ErrNoRows {
		return asymptoteobserve.LearningMemoryV1{}, false, nil
	}
	if err != nil {
		return asymptoteobserve.LearningMemoryV1{}, false, err
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
