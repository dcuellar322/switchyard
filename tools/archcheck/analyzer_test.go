package main

import "testing"

func TestAnalyzeRejectsDomainAdapterImport(t *testing.T) {
	t.Parallel()

	packages := []packageInfo{{
		ImportPath: modulePath + "/internal/catalog/domain",
		Imports:    []string{modulePath + "/internal/runtime/adapters"},
	}}
	violations := analyze(modulePath, packages)
	if len(violations) == 0 {
		t.Fatal("analyze() accepted deliberate forbidden import")
	}
}

func TestAnalyzeAllowsApplicationPort(t *testing.T) {
	t.Parallel()

	packages := []packageInfo{{
		ImportPath: modulePath + "/internal/catalog/application",
		Imports:    []string{modulePath + "/internal/catalog/domain"},
	}}
	if violations := analyze(modulePath, packages); len(violations) != 0 {
		t.Fatalf("analyze() violations = %#v", violations)
	}
}

func TestAnalyzeRejectsGenericRootPackage(t *testing.T) {
	t.Parallel()

	packages := []packageInfo{{ImportPath: modulePath + "/internal/utils"}}
	if violations := analyze(modulePath, packages); len(violations) != 1 {
		t.Fatalf("analyze() violations = %#v", violations)
	}
}

func TestAnalyzeRejectsPublicSDKImportingInternalImplementation(t *testing.T) {
	t.Parallel()

	packages := []packageInfo{{
		ImportPath: modulePath + "/sdk/plugin",
		Imports:    []string{modulePath + "/internal/plugins/application"},
	}}
	if violations := analyze(modulePath, packages); len(violations) != 1 {
		t.Fatalf("analyze() violations = %#v", violations)
	}
}
