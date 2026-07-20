package secretfile

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestValidateRequiresRegularProtectedFile(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	protected := filepath.Join(directory, "protected.key")
	if err := os.WriteFile(protected, []byte("key"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Validate(protected); err != nil {
		t.Fatalf("Validate(protected) = %v", err)
	}
	if err := Validate(directory); err == nil {
		t.Fatal("directory accepted as a private key")
	}
	if runtime.GOOS != "windows" {
		insecure := filepath.Join(directory, "insecure.key")
		if err := os.WriteFile(insecure, []byte("key"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := Validate(insecure); err == nil {
			t.Fatal("group-readable private key accepted")
		}
	}
}
