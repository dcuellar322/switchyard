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
	sort.Slice(violations, func(i, j int) bool {
		return violations[i].String() < violations[j].String()
	})
	return violations
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
	if strings.HasPrefix(importer, module+"/sdk/") && strings.HasPrefix(imported, module+"/internal/") {
		violations = append(violations, violation{
			Importer: importer,
			Imported: imported,
			Rule:     "public SDK packages cannot depend on internal implementation packages",
		})
	}
	if strings.Contains(importer, "/domain") {
		for _, forbidden := range []string{"/application", "/adapters", "/transport"} {
			if strings.HasPrefix(imported, module+"/") && strings.Contains(imported, forbidden) {
				violations = append(violations, violation{
					Importer: importer,
					Imported: imported,
					Rule:     "domain packages cannot depend on application or adapter layers",
				})
			}
		}
	}
	importerDomain := domainName(module, importer)
	importedDomain := domainName(module, imported)
	if importerDomain != "" && importedDomain != "" && importerDomain != importedDomain && strings.Contains(imported, "/adapters") {
		violations = append(violations, violation{
			Importer: importer,
			Imported: imported,
			Rule:     "a domain cannot import another domain's adapter",
		})
	}
	if strings.Contains(importer, "/transport/httpapi") {
		if imported == "database/sql" || strings.Contains(imported, "/runtime/compose") || strings.Contains(imported, "/runtime/process") || strings.Contains(imported, "docker") {
			violations = append(violations, violation{
				Importer: importer,
				Imported: imported,
				Rule:     "HTTP adapters cannot access persistence or runtime infrastructure directly",
			})
		}
	}
	if strings.Contains(importer, "/agents/mcp") {
		if imported == "database/sql" || imported == "os/exec" || strings.Contains(imported, "docker") {
			violations = append(violations, violation{
				Importer: importer,
				Imported: imported,
				Rule:     "MCP adapters must call application use cases",
			})
		}
	}
	return violations
}

func domainName(module, importPath string) string {
	relative := strings.TrimPrefix(importPath, module+"/internal/")
	if relative == importPath {
		return ""
	}
	name, _, _ := strings.Cut(relative, "/")
	switch name {
	case "actions", "agents", "catalog", "diagnostics", "discovery", "environments", "manifest", "observability", "operations", "plugins", "ports", "routing", "runtime", "sourcecontrol", "terminal", "workspace":
		return name
	default:
		return ""
	}
}
