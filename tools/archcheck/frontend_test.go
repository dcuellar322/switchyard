package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAnalyzeFrontendRejectsDirectHTTPDuplicateTypesAndGodComponents(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeArchitectureFixture(t, root, "api/generated/types.gen.ts", "export type Project = { id: string }\n")
	writeArchitectureFixture(t, root, "domains/projects/Bad.vue", `<script setup lang="ts">
type Project = { id: string }
fetch('/api/v1/projects')
</script>
<template><main /></template>
`+strings.Repeat("<template />\n", maximumVueLogicAndMarkupLines))

	violations, err := analyzeFrontend(root)
	if err != nil {
		t.Fatalf("analyzeFrontend() error = %v", err)
	}
	if len(violations) != 3 {
		t.Fatalf("analyzeFrontend() violations = %#v", violations)
	}
}

func TestAnalyzeFrontendAllowsGeneratedClientAndLocalViewModels(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeArchitectureFixture(t, root, "api/generated/types.gen.ts", "export type Project = { id: string }\n")
	writeArchitectureFixture(t, root, "api/generated/client.gen.ts", "fetch('/api/v1/projects')\n")
	writeArchitectureFixture(t, root, "domains/projects/Good.vue", `<script setup lang="ts">
type ProjectCard = { id: string }
</script>
<template><main /></template>
<style scoped>
`+strings.Repeat(".row {}\n", maximumVueLogicAndMarkupLines+1)+"</style>\n")

	violations, err := analyzeFrontend(root)
	if err != nil {
		t.Fatalf("analyzeFrontend() error = %v", err)
	}
	if len(violations) != 0 {
		t.Fatalf("analyzeFrontend() violations = %#v", violations)
	}
}

func writeArchitectureFixture(t *testing.T, root, relative, contents string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir fixture: %v", err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
}
