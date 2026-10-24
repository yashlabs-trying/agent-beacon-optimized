package learning

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/asymptote-labs/agent-beacon/pkg/asymptoteobserve"
)

func placementMemory(id, project, body string) asymptoteobserve.LearningMemoryV1 {
	return asymptoteobserve.LearningMemoryV1{
		ID: id, CandidateID: "c-" + id,
		Kind: asymptoteobserve.LearningMemoryKindConvention, Title: id, Body: body,
		Project:  asymptoteobserve.LearningProjectV1{ID: project},
		Tags:     []string{"source:verified"},
		Evidence: []asymptoteobserve.LearningEvidenceV1{{TraceID: "t-" + id}},
	}
}

func TestDatabasePathHonorsMemoryDirEnv(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(MemoryDirEnv, dir)
	if got := DatabasePath("/somewhere/runtime.jsonl"); got != filepath.Join(dir, storeFile) {
		t.Fatalf("DatabasePath = %q, want %q", got, filepath.Join(dir, storeFile))
	}
	t.Setenv(MemoryDirEnv, "")
	if got := DatabasePath("/data/beacon/endpoint/logs/runtime.jsonl"); got != "/data/beacon/endpoint/"+storeFile {
		t.Fatalf("default DatabasePath = %q", got)
	}
}

func TestCheckPlacementFlagsSynchronizedMount(t *testing.T) {
	if _, err := os.Stat("/mnt/c"); err != nil {
		t.Skip("no /mnt/c mount on this host")
	}
	p := CheckPlacement("/mnt/c/Users/example/memory.db")
	if !p.Synchronized || p.Warning == "" {
		t.Fatalf("expected a synchronized-mount warning for /mnt/c, got %+v", p)
	}
}

func TestCheckPlacementAllowsLocalDir(t *testing.T) {
	p := CheckPlacement(filepath.Join(t.TempDir(), "memory.db"))
	if p.Synchronized {
		t.Fatalf("local temp dir wrongly flagged synchronized: %+v", p)
	}
}

func TestMigrateDatabaseRoundTrip(t *testing.T) {
	src := filepath.Join(t.TempDir(), "src.db")
	store := Open(src)
	for i := 0; i < 25; i++ {
		if err := store.PutMemory(placementMemory(
			"m"+string(rune('a'+i%26))+string(rune('0'+i/26)), "p",
			"The component value is set.")); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(t.TempDir(), "migrated", "dst.db")
	if err := MigrateDatabase(src, dst); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if _, err := os.Stat(src); err != nil {
		t.Fatalf("source must be preserved after migration: %v", err)
	}
	moved := Open(dst)
	defer moved.Close()
	got, err := moved.ListMemories(Query{ProjectID: "p", Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 {
		t.Fatalf("migrated store returned no memories")
	}
}

func TestMigrateRefusesExistingDestination(t *testing.T) {
	src := filepath.Join(t.TempDir(), "src.db")
	store := Open(src)
	if err := store.PutMemory(placementMemory("m1", "p", "body")); err != nil {
		t.Fatal(err)
	}
	_ = store.Close()
	dst := filepath.Join(t.TempDir(), "dst.db")
	if err := os.WriteFile(dst, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := MigrateDatabase(src, dst); err == nil {
		t.Fatalf("expected refusal to overwrite an existing destination")
	}
}
