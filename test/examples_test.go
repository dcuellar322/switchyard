package test

import (
	"os"
	"path/filepath"
	"testing"

	manifestApplication "switchyard.dev/switchyard/internal/manifest/application"
)

func TestPublishedProjectExamplesValidate(t *testing.T) {
	t.Parallel()
	paths, err := filepath.Glob(filepath.Join("..", "examples", "projects", "*", ".switchyard", "project.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) < 2 {
		t.Fatalf("example manifests = %v", paths)
	}
	for _, path := range paths {
		path := path
		t.Run(filepath.Base(filepath.Dir(filepath.Dir(path))), func(t *testing.T) {
			t.Parallel()
			contents, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := manifestApplication.ParseYAML(contents); err != nil {
				t.Fatalf("%s: %v", path, err)
			}
		})
	}
}
