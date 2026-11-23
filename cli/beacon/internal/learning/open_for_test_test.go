package learning

import "testing"

// openForTest opens a Store that is closed when the test ends. Cleanups run in
// reverse order, so it closes before t.TempDir removes the directory; Windows
// cannot delete a database file that is still open.
func openForTest(t *testing.T, path string) *Store {
	t.Helper()
	s := Open(path)
	t.Cleanup(func() { _ = s.Close() })
	return s
}
