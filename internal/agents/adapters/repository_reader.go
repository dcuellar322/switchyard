// Package adapters implements provider-neutral agent infrastructure boundaries.
package adapters

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	discoveryDomain "switchyard.dev/switchyard/internal/discovery/domain"
)

const maxEvidenceSourceBytes = 1 << 20

// RepositoryReader returns line-bounded excerpts without executing repository content.
type RepositoryReader struct{}

// ReadExcerpt enforces root containment, regular files, source size, line range, and output size.
func (RepositoryReader) ReadExcerpt(ctx context.Context, root, relative string, location discoveryDomain.SourceRange, limit int64) (string, bool, error) {
	if err := validateExcerptRequest(root, relative, location, limit); err != nil {
		return "", false, err
	}
	file, err := openEvidenceSource(root, relative)
	if err != nil {
		return "", false, err
	}
	defer func() { _ = file.Close() }()
	return scanExcerpt(ctx, file, location, limit)
}

func validateExcerptRequest(root, relative string, location discoveryDomain.SourceRange, limit int64) error {
	if root == "" || relative == "" || filepath.IsAbs(relative) || location.StartLine < 1 || location.EndLine < location.StartLine || limit < 1 {
		return errors.New("invalid evidence excerpt request")
	}
	return nil
}

func openEvidenceSource(root, relative string) (*os.File, error) {
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, fmt.Errorf("resolve evidence root: %w", err)
	}
	candidate, err := filepath.EvalSymlinks(filepath.Join(canonicalRoot, filepath.Clean(relative)))
	if err != nil {
		return nil, err
	}
	contained, err := filepath.Rel(canonicalRoot, candidate)
	if err != nil || contained == ".." || strings.HasPrefix(contained, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("evidence path %q escapes repository root", relative)
	}
	file, err := os.Open(candidate)
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > maxEvidenceSourceBytes {
		_ = file.Close()
		return nil, fmt.Errorf("evidence source %q must be a regular file no larger than 1 MiB", relative)
	}
	return file, nil
}

func scanExcerpt(ctx context.Context, file *os.File, location discoveryDomain.SourceRange, limit int64) (string, bool, error) {
	scanner := bufio.NewScanner(io.LimitReader(file, maxEvidenceSourceBytes+1))
	scanner.Buffer(make([]byte, 64<<10), maxEvidenceSourceBytes)
	var result strings.Builder
	truncated := false
	for line := 1; scanner.Scan(); line++ {
		if err := ctx.Err(); err != nil {
			return "", false, err
		}
		if line < location.StartLine {
			continue
		}
		if line > location.EndLine {
			break
		}
		value := scanner.Text()
		required := int64(len(value) + 1)
		if int64(result.Len())+required > limit {
			remaining := int(limit - int64(result.Len()))
			if remaining > 0 {
				result.WriteString(value[:min(len(value), remaining)])
			}
			truncated = true
			break
		}
		result.WriteString(value)
		result.WriteByte('\n')
	}
	if err := scanner.Err(); err != nil {
		return "", false, err
	}
	return strings.TrimSuffix(result.String(), "\n"), truncated, nil
}
