//go:build windows

package learning

// Windows key-file protection: the Ed25519 private key bytes are encrypted with
// DPAPI (CryptProtectData) in the CURRENT-USER scope. Only the protected blob is
// written to disk; decryption requires the same Windows user on the same machine.
// There is no plaintext fallback: a decryption failure is an error and the key is
// never regenerated. Machine-wide scope is deliberately not used.

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

var (
	modCrypt32    = syscall.NewLazyDLL("crypt32.dll")
	procProtect   = modCrypt32.NewProc("CryptProtectData")
	procUnprotect = modCrypt32.NewProc("CryptUnprotectData")
	modKernel32   = syscall.NewLazyDLL("kernel32.dll")
	procLocalFree = modKernel32.NewProc("LocalFree")
)

type dataBlob struct {
	cbData uint32
	pbData *byte
}

// CRYPTPROTECT_UI_FORBIDDEN: never prompt for UI (required for a service).
const cryptProtectUIForbidden = 0x1

func newBlob(data []byte) (dataBlob, error) {
	if len(data) == 0 {
		return dataBlob{}, fmt.Errorf("DPAPI: empty input")
	}
	return dataBlob{cbData: uint32(len(data)), pbData: &data[0]}, nil
}

func encodeKeyBlob(priv []byte) ([]byte, error) {
	in, err := newBlob(priv)
	if err != nil {
		return nil, err
	}
	var out dataBlob
	ret, _, callErr := procProtect.Call(
		uintptr(unsafe.Pointer(&in)),
		0, // szDataDescr
		0, // pOptionalEntropy
		0, // pvReserved
		0, // pPromptStruct
		uintptr(cryptProtectUIForbidden),
		uintptr(unsafe.Pointer(&out)),
	)
	if ret == 0 {
		return nil, fmt.Errorf("DPAPI CryptProtectData failed: %w", callErr)
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(out.pbData)))
	return append([]byte(nil), unsafe.Slice(out.pbData, out.cbData)...), nil
}

func decodeKeyBlob(data []byte) ([]byte, error) {
	in, err := newBlob(data)
	if err != nil {
		return nil, err
	}
	var out dataBlob
	ret, _, callErr := procUnprotect.Call(
		uintptr(unsafe.Pointer(&in)),
		0, // ppszDataDescr
		0, // pOptionalEntropy
		0, // pvReserved
		0, // pPromptStruct
		uintptr(cryptProtectUIForbidden),
		uintptr(unsafe.Pointer(&out)),
	)
	if ret == 0 {
		return nil, fmt.Errorf("DPAPI CryptUnprotectData failed: %w", callErr)
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(out.pbData)))
	return append([]byte(nil), unsafe.Slice(out.pbData, out.cbData)...), nil
}

// checkKeyFilePerms on Windows relies on DPAPI for confidentiality; filesystem
// ACL hardening is documented but not asserted here (see the report).
func checkKeyFilePerms(path string) error {
	if _, err := os.Stat(path); err != nil {
		return err
	}
	return nil
}
