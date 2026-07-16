package main

import (
	"fmt"
	"sort"
	"strings"
)

type packageInfo struct {
	ImportPath string
	Imports    []string
}

type violation struct {
	Importer string
	Imported string
	Rule     string
}

func (v violation) String() string {
	return fmt.Sprintf("%s imports %s: %s", v.Importer, v.Imported, v.Rule)
}

func analyze(module string, packages []packageInfo) []violation {
	var violations []violation
	for _, current := range packages {
		if !strings.HasPrefix(current.ImportPath, module+"/") {
			continue
		}
		violations = append(violations, forbiddenPackageName(module, current.ImportPath)...)
		for _, imported := range current.Imports {
			violations = append(violations, checkImport(module, current.ImportPath, imported)...)
		}
	}
	sortViolations(violations)
	return violations
}

func sortViolations(violations []violation) {
	sort.Slice(violations, func(i, j int) bool {
		return violations[i].String() < violations[j].String()
	})
}

func forbiddenPackageName(module, importPath string) []violation {
	relative := strings.TrimPrefix(importPath, module+"/")
	parts := strings.Split(relative, "/")
	if len(parts) != 2 || parts[0] != "internal" {
		return nil
	}
	switch parts[1] {
	case "utils", "common", "helpers":
		return []violation{{
			Importer: importPath,
			Imported: "-",
			Rule:     "generic internal root packages are forbidden",
		}}
	default:
		return nil
	}
}

func checkImport(module, importer, imported string) []violation {
	var violations []violation
	violations = append(violations, checkSDKBoundary(module, importer, imported)...)
	violations = append(violations, checkLayerDirection(module, importer, imported)...)
	violations = append(violations, checkCrossDomainAdapter(module, importer, imported)...)
	violations = append(violations, checkHTTPBoundary(importer, imported)...)
	violations = append(violations, checkMCPBoundary(module, importer, imported)...)
	return violations
}

func checkSDKBoundary(module, importer, imported string) []violation {
	if strings.HasPrefix(importer, module+"/sdk/") && strings.HasPrefix(imported, module+"/internal/") {
		return []violation{{
			Importer: importer,
			Imported: imported,
			Rule:     "public SDK packages cannot depend on internal implementation packages",
		}}
	}
	return nil
}

func checkLayerDirection(module, importer, imported string) []violation {
	var violations []violation
	if isLayer(importer, "domain") {
		if strings.HasPrefix(imported, module+"/") &&
			(isLayer(imported, "application") || isLayer(imported, "adapters") || strings.Contains(imported, "/transport/")) {
			violations = append(violations, violation{
				Importer: importer, Imported: imported,
				Rule: "domain packages cannot depend on application or adapter layers",
			})
		}
		if forbiddenDomainInfrastructure(module, imported) {
			violations = append(violations, violation{
				Importer: importer, Imported: imported,
				Rule: "domain packages cannot depend on infrastructure or third-party packages",
			})
		}
	}
	if isLayer(importer, "application") && strings.HasPrefix(imported, module+"/") &&
		(isLayer(imported, "adapters") || strings.Contains(imported, "/transport/")) {
		violations = append(violations, violation{
			Importer: importer, Imported: imported,
			Rule: "application packages cannot depend on adapter or transport layers",
		})
	}
	return violations
}

func checkCrossDomainAdapter(module, importer, imported string) []violation {
	importerDomain := domainName(module, importer)
	importedDomain := domainName(module, imported)
	if importerDomain != "" && importedDomain != "" && importerDomain != importedDomain && strings.Contains(imported, "/adapters") {
		return []violation{{
			Importer: importer,
			Imported: imported,
			Rule:     "a domain cannot import another domain's adapter",
		}}
	}
	return nil
}

func checkHTTPBoundary(importer, imported string) []violation {
	if strings.Contains(importer, "/transport/httpapi") {
		if imported == "database/sql" || strings.Contains(imported, "/runtime/compose") || strings.Contains(imported, "/runtime/process") || strings.Contains(imported, "docker") {
			return []violation{{
				Importer: importer,
				Imported: imported,
				Rule:     "HTTP adapters cannot access persistence or runtime infrastructure directly",
			}}
		}
	}
	return nil
}

func checkMCPBoundary(module, importer, imported string) []violation {
	if strings.Contains(importer, "/transport/mcpserver") {
		if imported == "database/sql" || imported == "os/exec" || strings.Contains(imported, "docker") ||
			(strings.HasPrefix(imported, module+"/internal/") &&
				(strings.Contains(imported, "/adapters") || strings.Contains(imported, "/platform/") || strings.Contains(imported, "/runtime/"))) {
			return []violation{{
				Importer: importer,
				Imported: imported,
				Rule:     "MCP adapters must call application use cases",
			}}
		}
	}
	return nil
}

func forbiddenDomainInfrastructure(module, imported string) bool {
	if strings.HasPrefix(imported, module+"/") {
		return false
	}
	if imported == "database/sql" || imported == "net/http" || imported == "os/exec" {
		return true
	}
	// Standard-library packages have no dot in their first path segment. A
	// domain importing any third-party package has crossed an infrastructure
	// boundary and must instead receive a domain value through an application
	// port.
	first, _, _ := strings.Cut(imported, "/")
	return strings.Contains(first, ".")
}

func isLayer(importPath, layer string) bool {
	parts := strings.Split(importPath, "/")
	for _, part := range parts {
		if part == layer {
			return true
		}
	}
	return false
}

func domainName(module, importPath string) string {
	relative := strings.TrimPrefix(importPath, module+"/internal/")
	if relative == importPath {
		return ""
	}
	parts := strings.Split(relative, "/")
	if len(parts) < 2 {
		return ""
	}
	switch parts[1] {
	case "domain", "application", "adapters":
		return parts[0]
	default:
		return ""
	}
}
