package application

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const maxDiscoveryFileSize = 1024 * 1024

// Root is a user-selected, canonical repository boundary.
type Root struct{ Path string }

// SelectRoot validates a repository root without reading repository content.
func SelectRoot(path string) (Root, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return Root{}, fmt.Errorf("resolve repository root: %w", err)
	}
	canonical, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return Root{}, fmt.Errorf("resolve repository root symlinks: %w", err)
	}
	info, err := os.Stat(canonical)
	if err != nil {
		return Root{}, fmt.Errorf("inspect repository root: %w", err)
	}
	if !info.IsDir() {
		return Root{}, errors.New("repository root must be a directory")
	}
	return Root{Path: canonical}, nil
}

// ReadFile reads one known relative file while enforcing size and containment.
func (r Root) ReadFile(relative string) ([]byte, error) {
	if relative == "" || filepath.IsAbs(relative) || relative == ".env" || strings.HasPrefix(relative, ".env.") && relative != ".env.example" {
		return nil, fmt.Errorf("discovery file %q is not allowed", relative)
	}
	path := filepath.Join(r.Path, filepath.Clean(relative))
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil, err
	}
	if err := contained(r.Path, resolved); err != nil {
		return nil, err
	}
	file, err := os.Open(resolved)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > maxDiscoveryFileSize {
		return nil, fmt.Errorf("discovery file %q must be a regular file no larger than 1 MiB", relative)
	}
	contents, err := io.ReadAll(io.LimitReader(file, maxDiscoveryFileSize+1))
	if err != nil {
		return nil, err
	}
	return contents, nil
}

func contained(root, candidate string) error {
	relative, err := filepath.Rel(root, candidate)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("path %q escapes the selected repository root", candidate)
	}
	return nil
}
