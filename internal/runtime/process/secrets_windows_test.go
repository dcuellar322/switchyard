//go:build windows

package process

import "testing"

func TestDecodeCredentialBlobSupportsUTF16AndOpaqueUTF8(t *testing.T) {
	t.Parallel()
	if got := decodeCredentialBlob([]byte{'s', 0, 'e', 0, 'c', 0, 'r', 0, 'e', 0, 't', 0}); got != "secret" {
		t.Fatalf("UTF-16 value = %q", got)
	}
	if got := decodeCredentialBlob([]byte("token-value")); got != "token-value" {
		t.Fatalf("UTF-8 value = %q", got)
	}
}
