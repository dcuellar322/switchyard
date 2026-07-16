package application

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"switchyard.dev/switchyard/internal/discovery/domain"
	"switchyard.dev/switchyard/internal/foundation/identifier"
)

// Scanner independently emits evidence without executing repository code.
type Scanner interface {
	Name() string
	Scan(context.Context, Root) ([]domain.Evidence, error)
}

// ScanAll runs scanners in stable registration order and assigns evidence IDs.
func ScanAll(ctx context.Context, root Root, scanners []Scanner) ([]domain.Evidence, error) {
	var result []domain.Evidence
	for _, scanner := range scanners {
		items, err := scanner.Scan(ctx, root)
		if err != nil {
			return nil, fmt.Errorf("%s scanner: %w", scanner.Name(), err)
		}
		for index := range items {
			items[index].Scanner = scanner.Name()
			items[index].Warnings = nonNil(items[index].Warnings)
			if !json.Valid(items[index].Data) {
				return nil, fmt.Errorf("%s scanner emitted invalid JSON", scanner.Name())
			}
			id, err := identifier.New("evidence")
			if err != nil {
				return nil, err
			}
			items[index].ID = id
		}
		result = append(result, items...)
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].SourcePath == result[j].SourcePath {
			return result[i].Location.StartLine < result[j].Location.StartLine
		}
		return result[i].SourcePath < result[j].SourcePath
	})
	return result, nil
}

func nonNil(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}
