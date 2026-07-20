package adapters

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"switchyard.dev/switchyard/internal/support/domain"
)

// ArchiveWriter atomically writes a private, redacted support ZIP.
type ArchiveWriter struct{}

// Write persists the already-reviewed preview without adding implicit files.
func (ArchiveWriter) Write(output string, preview domain.Preview) (domain.BundleReceipt, error) {
	absolute, err := filepath.Abs(output)
	if err != nil {
		return domain.BundleReceipt{}, fmt.Errorf("resolve support bundle output: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(absolute), 0o700); err != nil {
		return domain.BundleReceipt{}, fmt.Errorf("create support bundle directory: %w", err)
	}
	if _, err := os.Lstat(absolute); err == nil {
		return domain.BundleReceipt{}, fmt.Errorf("support bundle already exists: %s", absolute)
	} else if !os.IsNotExist(err) {
		return domain.BundleReceipt{}, fmt.Errorf("inspect support bundle output: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(absolute), ".switchyard-support-*.tmp")
	if err != nil {
		return domain.BundleReceipt{}, fmt.Errorf("create support bundle: %w", err)
	}
	temporaryPath := temporary.Name()
	defer func() {
		_ = temporary.Close()
		_ = os.Remove(temporaryPath)
	}()
	if err := temporary.Chmod(0o600); err != nil {
		return domain.BundleReceipt{}, fmt.Errorf("restrict support bundle: %w", err)
	}

	files := []string{"manifest.json", "internal-errors.ndjson"}
	manifest := domain.BundleManifest{SchemaVersion: domain.BundleSchema, Preview: preview, Files: files}
	archive := zip.NewWriter(temporary)
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return domain.BundleReceipt{}, fmt.Errorf("encode support manifest: %w", err)
	}
	if err := writeArchiveFile(archive, files[0], append(manifestBytes, '\n')); err != nil {
		return domain.BundleReceipt{}, err
	}
	var errorsJSON []byte
	for _, entry := range preview.InternalErrors {
		encoded, marshalErr := json.Marshal(entry)
		if marshalErr != nil {
			return domain.BundleReceipt{}, fmt.Errorf("encode internal error: %w", marshalErr)
		}
		errorsJSON = append(errorsJSON, encoded...)
		errorsJSON = append(errorsJSON, '\n')
	}
	if err := writeArchiveFile(archive, files[1], errorsJSON); err != nil {
		return domain.BundleReceipt{}, err
	}
	if err := archive.Close(); err != nil {
		return domain.BundleReceipt{}, fmt.Errorf("finalize support bundle: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return domain.BundleReceipt{}, fmt.Errorf("sync support bundle: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return domain.BundleReceipt{}, fmt.Errorf("close support bundle: %w", err)
	}
	if err := commitExclusive(temporaryPath, absolute); err != nil {
		return domain.BundleReceipt{}, fmt.Errorf("commit support bundle: %w", err)
	}
	contents, err := os.Open(absolute)
	if err != nil {
		return domain.BundleReceipt{}, fmt.Errorf("verify support bundle: %w", err)
	}
	hash := sha256.New()
	size, copyErr := io.Copy(hash, contents)
	closeErr := contents.Close()
	if copyErr != nil || closeErr != nil {
		return domain.BundleReceipt{}, fmt.Errorf("hash support bundle: %w", errorsJoin(copyErr, closeErr))
	}
	return domain.BundleReceipt{Path: absolute, SHA256: hex.EncodeToString(hash.Sum(nil)), SizeBytes: size, Preview: preview}, nil
}

func writeArchiveFile(archive *zip.Writer, name string, contents []byte) error {
	header := &zip.FileHeader{Name: name, Method: zip.Deflate}
	header.SetMode(0o600)
	writer, err := archive.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("create support archive entry %s: %w", name, err)
	}
	if _, err := writer.Write(contents); err != nil {
		return fmt.Errorf("write support archive entry %s: %w", name, err)
	}
	return nil
}

func errorsJoin(values ...error) error {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}
