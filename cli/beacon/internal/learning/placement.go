package learning

// Phase 1 (V7 hardening): safe, predictable database placement.
//
// A SQLite store must live on a local filesystem. On WSL, a database under a
// Windows drive mount (/mnt/c, drvfs) or any synchronized/network filesystem
// (/mnt/c OneDrive, 9p, cifs, nfs, fuse remotes) pays per-commit latency of the
// remote/synced filesystem (observed ~1 s/commit, 16-client write p99 ~7.8 s)
// instead of the ~ms of a local disk. This file:
//
//   - resolves the effective database path, honoring an explicit local directory
//     override (BEACON_MEMORY_DIR) so the source worktree and the runtime
//     database are independent;
//   - detects unsuitable (synchronized/network) filesystems and returns a clear
//     warning rather than failing silently;
//   - provides a transactionally consistent migration (VACUUM INTO + integrity
//     and row-count verification) that never deletes the source before the
//     destination is verified.
//
// It never relocates a database on its own. Placement changes are explicit.

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// MemoryDirEnv overrides the directory that holds the runtime database. It lets an
// operator place the store on a local filesystem independent of the source
// worktree or the runtime log location.
const MemoryDirEnv = "BEACON_MEMORY_DIR"

// Placement describes where the database lives and whether that location is
// suitable for a write-heavy SQLite workload.
type Placement struct {
	Path         string `json:"path"`
	Filesystem   string `json:"filesystem,omitempty"`
	Synchronized bool   `json:"synchronized"`
	Warning      string `json:"warning,omitempty"`
}

// DatabasePath resolves the effective database path for a runtime log path.
// An explicit BEACON_MEMORY_DIR takes precedence; otherwise the store lives next
// to the runtime log (the historical, compatible location).
func DatabasePath(runtimeLogPath string) string {
	if dir := strings.TrimSpace(os.Getenv(MemoryDirEnv)); dir != "" {
		return filepath.Join(dir, storeFile)
	}
	return PathForRuntimeLog(runtimeLogPath)
}

// CheckPlacement inspects the filesystem that holds the database path and returns
// a warning when it is synchronized or network-mounted. It never moves anything.
func CheckPlacement(dbPath string) Placement {
	p := Placement{Path: dbPath}
	fs, sync := filesystemOf(dbPath)
	p.Filesystem = fs
	p.Synchronized = sync
	if sync {
		p.Warning = fmt.Sprintf(
			"database %q is on a synchronized or network filesystem (%s); SQLite writes there are far slower and risk lock/consistency problems. Move it to a local directory (set %s to a path on local disk) before production use.",
			dbPath, fs, MemoryDirEnv)
	}
	return p
}

// filesystemOf returns the filesystem type holding path and whether it is a
// synchronized or network mount. Detection is best-effort and platform-specific.
func filesystemOf(path string) (string, bool) {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	switch runtime.GOOS {
	case "linux":
		return linuxFilesystemOf(abs)
	case "windows":
		if strings.HasPrefix(abs, `\\`) {
			return "UNC/network", true
		}
		return "local", false
	default:
		return "", false
	}
}

// synchronizedFilesystems are mount types that must not host a SQLite store.
var synchronizedFilesystems = map[string]bool{
	"drvfs": true, "9p": true, "cifs": true, "smb3": true, "smbfs": true,
	"nfs": true, "nfs4": true, "vboxsf": true, "fuse.rclone": true,
	"fuse.sshfs": true, "fuse.gvfsd-fuse": true, "fuseblk": true, "afs": true,
}

func linuxFilesystemOf(abs string) (string, bool) {
	data, err := os.ReadFile("/proc/mounts")
	if err != nil {
		// Fall back to the well-known WSL Windows-drive mounts.
		if strings.HasPrefix(abs, "/mnt/") && len(abs) > len("/mnt/") {
			return "drvfs", true
		}
		return "", false
	}
	bestPoint, bestType := "", ""
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		point, fsType := unescapeMountField(fields[1]), fields[2]
		if abs == point || strings.HasPrefix(abs, strings.TrimRight(point, "/")+"/") {
			if len(point) > len(bestPoint) {
				bestPoint, bestType = point, fsType
			}
		}
	}
	// fuse.* mounts report as "fuse.<subtype>" or "fuse"; treat any fuse remote as
	// synchronized unless it is a known-local subtype.
	if bestType == "" {
		return "", false
	}
	if synchronizedFilesystems[bestType] || strings.HasPrefix(bestType, "fuse.") {
		return bestType, true
	}
	return bestType, false
}

func unescapeMountField(s string) string {
	repl := strings.NewReplacer(`\040`, " ", `\011`, "\t", `\012`, "\n", `\134`, `\`)
	return repl.Replace(s)
}

// MigrateDatabase copies the SQLite database at src to dst using VACUUM INTO,
// which produces a transactionally consistent copy that includes committed WAL
// contents (unlike a naive file copy). It then verifies integrity and that the
// memory row count matches, and only reports success if both hold. The source is
// never modified or deleted.
func MigrateDatabase(src, dst string) error {
	if strings.TrimSpace(src) == "" || strings.TrimSpace(dst) == "" {
		return fmt.Errorf("migrate: source and destination are required")
	}
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("migrate: source database: %w", err)
	}
	if _, err := os.Stat(dst); err == nil {
		return fmt.Errorf("migrate: destination %q already exists; refusing to overwrite", dst)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return fmt.Errorf("migrate: create destination dir: %w", err)
	}

	srcDB, err := sql.Open("sqlite", src)
	if err != nil {
		return fmt.Errorf("migrate: open source: %w", err)
	}
	defer srcDB.Close()

	before, err := memoryRowCount(srcDB)
	if err != nil {
		return fmt.Errorf("migrate: count source memories: %w", err)
	}
	// VACUUM INTO writes a consistent snapshot, including WAL contents.
	if _, err := srcDB.Exec(`VACUUM INTO ?`, dst); err != nil {
		// Clean up a partial destination so a retry is not blocked by a corrupt
		// leftover; the source is untouched.
		_ = os.Remove(dst)
		return fmt.Errorf("migrate: vacuum into destination: %w", err)
	}

	dstDB, err := sql.Open("sqlite", dst)
	if err != nil {
		return fmt.Errorf("migrate: open destination: %w", err)
	}
	defer dstDB.Close()
	var integrity string
	if err := dstDB.QueryRow(`PRAGMA integrity_check`).Scan(&integrity); err != nil {
		return fmt.Errorf("migrate: integrity check: %w", err)
	}
	if integrity != "ok" {
		return fmt.Errorf("migrate: destination integrity check failed: %s", integrity)
	}
	after, err := memoryRowCount(dstDB)
	if err != nil {
		return fmt.Errorf("migrate: count destination memories: %w", err)
	}
	if after != before {
		return fmt.Errorf("migrate: row count mismatch: source=%d destination=%d", before, after)
	}
	return nil
}

func memoryRowCount(db *sql.DB) (int, error) {
	var n int
	err := db.QueryRow(`SELECT COUNT(*) FROM memories`).Scan(&n)
	if err != nil && strings.Contains(err.Error(), "no such table") {
		return 0, nil
	}
	return n, err
}
