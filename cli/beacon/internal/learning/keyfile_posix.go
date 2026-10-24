//go:build !windows

package learning

// POSIX key-file protection: owner-only permissions (0600) with base64 key
// bytes on disk. A group- or world-accessible key is rejected on load.

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"
)

func encodeKeyBlob(priv []byte) ([]byte, error) {
	return []byte(base64.StdEncoding.EncodeToString(priv) + "\n"), nil
}

func decodeKeyBlob(data []byte) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(data)))
	if err != nil {
		return nil, fmt.Errorf("signing key: malformed encoding: %w", err)
	}
	return raw, nil
}

func checkKeyFilePerms(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("signing key %s is group/world accessible (%04o); expected 0600", path, info.Mode().Perm())
	}
	return nil
}
