package learning

// P6 tests: append-only security audit log. Lifecycle events are written in the
// mutation transaction; denials are best-effort. A missing audit write rolls the
// mutation back.

import (
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/asymptote-labs/agent-beacon/pkg/asymptoteobserve"
)

func p6Events(t *testing.T, s *Store) map[string]int {
	t.Helper()
	entries, err := s.AuditEntries()
	if err != nil {
		t.Fatal(err)
	}
	m := map[string]int{}
	for _, e := range entries {
		m[e.Event]++
	}
	return m
}

func TestP6LifecycleEventsRecorded(t *testing.T) {
	dbPath := filepath.Join(supTempDir(t), "memory.db")
	s := Open(dbPath)
	defer supClose(s)

	// approve
	_ = s.PutCandidate(supCandidate("c-appr"))
	if _, _, err := ApproveCandidate(s, "c-appr", "ok"); err != nil {
		t.Fatal(err)
	}
	// reject
	_ = s.PutCandidate(supCandidate("c-rej"))
	if _, err := RejectCandidate(s, "c-rej", "no"); err != nil {
		t.Fatal(err)
	}
	// supersede (approve then supersede)
	_ = s.PutCandidate(supCandidate("c-sup"))
	if _, _, err := ApproveCandidate(s, "c-sup", "ok"); err != nil {
		t.Fatal(err)
	}
	_ = s.PutMemory(asymptoteobserve.LearningMemoryV1{ID: "repl", CandidateID: "cr",
		Kind: asymptoteobserve.LearningMemoryKindConvention, Title: "t", Body: "b",
		Project: asymptoteobserve.LearningProjectV1{ID: "proj"}})
	if _, err := SupersedeCandidate(s, "c-sup", "repl", "x"); err != nil {
		t.Fatal(err)
	}

	ev := p6Events(t, s)
	if ev["memory_approved"] != 2 || ev["memory_rejected"] != 1 || ev["memory_superseded"] != 1 {
		t.Fatalf("audit events = %v", ev)
	}
}

func TestP6AuthorizationDenialAudited(t *testing.T) {
	dbPath := filepath.Join(supTempDir(t), "memory.db")
	seed := Open(dbPath)
	_ = seed.PutCandidate(p3Candidate("c-b", "project-B"))
	supClose(seed)

	s := OpenAuthorized(dbPath, NewProjectAllowlist("project-A"))
	defer supClose(s)
	_, _, _ = ApproveCandidate(s, "c-b", "x")
	if ev := p6Events(t, s); ev["authorization_denied"] != 1 {
		t.Fatalf("denial not audited: %v", ev)
	}
}

func TestP6NoFalseSuccessOnRollback(t *testing.T) {
	dbPath := filepath.Join(supTempDir(t), "memory.db")
	s := Open(dbPath)
	defer supClose(s)
	_ = s.PutCandidate(supCandidate("c1"))
	if _, _, err := ApproveCandidate(s, "c1", "ok"); err != nil {
		t.Fatal(err)
	}
	// second approval fails (not reviewable); no second success event
	_, _, _ = ApproveCandidate(s, "c1", "again")
	if ev := p6Events(t, s); ev["memory_approved"] != 1 {
		t.Fatalf("false success audit: %v", ev)
	}
}

func TestP6AuditFailureRollsBackMutation(t *testing.T) {
	dbPath := filepath.Join(supTempDir(t), "memory.db")
	s := Open(dbPath)
	defer supClose(s)
	db, _ := s.db()
	if _, err := db.Exec(`DROP TABLE audit`); err != nil {
		t.Fatal(err)
	}
	_ = s.PutCandidate(supCandidate("c1"))
	if _, _, err := ApproveCandidate(s, "c1", "ok"); err == nil {
		t.Fatal("expected the mutation to fail when the audit write fails")
	}
	got, _, _ := s.GetCandidate("c1")
	if got.State != asymptoteobserve.LearningCandidateStateCandidate {
		t.Fatalf("mutation committed without its audit record: state=%s", got.State)
	}
}

func TestP6OrderingAndRestart(t *testing.T) {
	dbPath := filepath.Join(supTempDir(t), "memory.db")
	s := Open(dbPath)
	for i := 0; i < 20; i++ {
		id := "c" + string(rune('a'+i))
		_ = s.PutCandidate(supCandidate(id))
		_, _, _ = ApproveCandidate(s, id, "ok")
	}
	supClose(s)

	r := Open(dbPath)
	defer supClose(r)
	entries, err := r.AuditEntries()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 20 {
		t.Fatalf("audit rows = %d, want 20", len(entries))
	}
	for i := 1; i < len(entries); i++ {
		if entries[i].Seq <= entries[i-1].Seq {
			t.Fatalf("audit seq not increasing at %d", i)
		}
	}
}

func TestP6ConcurrentAudit(t *testing.T) {
	dbPath := filepath.Join(supTempDir(t), "memory.db")
	s := Open(dbPath)
	defer supClose(s)
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		id := "c" + string(rune('a'+i))
		_ = s.PutCandidate(supCandidate(id))
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			_, _, _ = ApproveCandidate(s, id, "ok")
		}(id)
	}
	wg.Wait()
	if ev := p6Events(t, s); ev["memory_approved"] != 16 {
		t.Fatalf("concurrent audit events = %v, want 16 approvals", ev)
	}
}

func TestP6NoSensitiveContentInAudit(t *testing.T) {
	dbPath := filepath.Join(supTempDir(t), "memory.db")
	s := Open(dbPath)
	defer supClose(s)
	c := supCandidate("c1")
	c.Body = "SECRET_CANARY_BODY_VALUE"
	_ = s.PutCandidate(c)
	_, _, _ = ApproveCandidate(s, "c1", "ok")
	entries, _ := s.AuditEntries()
	for _, e := range entries {
		if strings.Contains(e.Detail, "SECRET_CANARY_BODY_VALUE") || strings.Contains(e.Actor, "SECRET_CANARY") {
			t.Fatalf("audit leaked content: %+v", e)
		}
	}
}
