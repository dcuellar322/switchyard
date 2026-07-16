package main

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const maximumVueLogicAndMarkupLines = 250

var (
	typeDeclarationPattern = regexp.MustCompile(`(?m)^\s*(?:export\s+)?(?:type|interface)\s+([A-Z][A-Za-z0-9_]*)\b`)
	directFetchPattern     = regexp.MustCompile(`\bfetch\s*\(`)
)

func analyzeFrontend(root string) ([]violation, error) {
	generatedTypes, err := os.ReadFile(filepath.Join(root, "api", "generated", "types.gen.ts"))
	if err != nil {
		return nil, fmt.Errorf("read generated frontend types: %w", err)
	}
	canonicalTypes := declaredTypeNames(generatedTypes)

	var violations []violation
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		relative, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		if strings.HasPrefix(filepath.ToSlash(relative), "api/generated/") {
			return nil
		}
		extension := filepath.Ext(path)
		if extension != ".ts" && extension != ".vue" {
			return nil
		}
		contents, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		name := filepath.ToSlash(path)
		if directFetchPattern.Match(contents) || strings.Contains(string(contents), "/api/v1") {
			violations = append(violations, violation{Importer: name, Imported: "HTTP", Rule: "frontend code must use the generated API client for documented endpoints"})
		}
		for declared := range declaredTypeNames(contents) {
			if canonicalTypes[declared] {
				violations = append(violations, violation{Importer: name, Imported: declared, Rule: "frontend API types must be imported from generated contracts, not handwritten"})
			}
		}
		if extension == ".vue" {
			lines, countErr := vueLogicAndMarkupLines(contents)
			if countErr != nil {
				return fmt.Errorf("inspect %s: %w", path, countErr)
			}
			if lines > maximumVueLogicAndMarkupLines {
				violations = append(violations, violation{
					Importer: name,
					Imported: "-",
					Rule:     fmt.Sprintf("Vue SFC has %d non-style lines; split cohesive responsibilities at %d", lines, maximumVueLogicAndMarkupLines),
				})
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk frontend architecture: %w", err)
	}
	sortViolations(violations)
	return violations, nil
}

func declaredTypeNames(contents []byte) map[string]bool {
	result := make(map[string]bool)
	for _, match := range typeDeclarationPattern.FindAllSubmatch(contents, -1) {
		result[string(match[1])] = true
	}
	return result
}

func vueLogicAndMarkupLines(contents []byte) (int, error) {
	scanner := bufio.NewScanner(strings.NewReader(string(contents)))
	lines := 0
	inStyle := false
	for scanner.Scan() {
		line := scanner.Text()
		if !inStyle && strings.Contains(line, "<style") {
			inStyle = !strings.Contains(line, "</style>")
			continue
		}
		if inStyle {
			if strings.Contains(line, "</style>") {
				inStyle = false
			}
			continue
		}
		lines++
	}
	if err := scanner.Err(); err != nil {
		return 0, err
	}
	return lines, nil
}
