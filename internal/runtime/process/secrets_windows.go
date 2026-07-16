//go:build windows

package process

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"

	"switchyard.dev/switchyard/internal/runtime/domain"
)

const credentialTypeGeneric = 1

type windowsCredential struct {
	Flags              uint32
	Type               uint32
	TargetName         *uint16
	Comment            *uint16
	LastWritten        windows.Filetime
	CredentialBlobSize uint32
	CredentialBlob     *byte
	Persist            uint32
	AttributeCount     uint32
	Attributes         uintptr
	TargetAlias        *uint16
	UserName           *uint16
}

func resolveKeychain(ctx context.Context, reference domain.SecretReference) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	target, err := windows.UTF16PtrFromString(reference.Key)
	if err != nil {
		return "", err
	}
	credential, release, err := readWindowsCredential(target)
	if err != nil {
		return "", fmt.Errorf("read Windows credential %q: %w", reference.Key, err)
	}
	defer release()
	if reference.Account != "" && credential.UserName != nil && !strings.EqualFold(windows.UTF16PtrToString(credential.UserName), reference.Account) {
		return "", errors.New("Windows credential account does not match the manifest reference")
	}
	if credential.CredentialBlob == nil || credential.CredentialBlobSize == 0 {
		return "", errors.New("Windows credential contains no secret value")
	}
	value := append([]byte(nil), unsafe.Slice(credential.CredentialBlob, credential.CredentialBlobSize)...)
	return decodeCredentialBlob(value), nil
}

func readWindowsCredential(target *uint16) (*windowsCredential, func(), error) {
	credentialManager := windows.NewLazySystemDLL("advapi32.dll")
	read := credentialManager.NewProc("CredReadW")
	free := credentialManager.NewProc("CredFree")
	var credential *windowsCredential
	result, _, callErr := read.Call(
		uintptr(unsafe.Pointer(target)),
		credentialTypeGeneric,
		0,
		uintptr(unsafe.Pointer(&credential)),
	)
	if result == 0 {
		return nil, func() {}, callErr
	}
	return credential, func() { _, _, _ = free.Call(uintptr(unsafe.Pointer(credential))) }, nil
}

func decodeCredentialBlob(value []byte) string {
	if len(value)%2 != 0 || len(value) == 0 {
		return string(value)
	}
	encoded := make([]uint16, len(value)/2)
	zeroHighBytes := 0
	for index := range encoded {
		encoded[index] = uint16(value[index*2]) | uint16(value[index*2+1])<<8
		if value[index*2+1] == 0 {
			zeroHighBytes++
		}
	}
	if zeroHighBytes*4 < len(encoded)*3 {
		return string(value)
	}
	return string(utf16.Decode(encoded))
}
