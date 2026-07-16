package adapters

import (
	"context"
	"errors"
	"testing"

	"switchyard.dev/switchyard/internal/environments/domain"
)

func TestMemoryRepositoryReplacesOnlySelectedProject(t *testing.T) {
	t.Parallel()

	repository := NewMemoryRepository()
	if err := repository.ReplaceProject(context.Background(), "one", []domain.Environment{{ID: "env-one", ProjectID: "one"}}); err != nil {
		t.Fatal(err)
	}
	if err := repository.ReplaceProject(context.Background(), "two", []domain.Environment{{ID: "env-two", ProjectID: "two"}}); err != nil {
		t.Fatal(err)
	}
	if err := repository.ReplaceProject(context.Background(), "one", []domain.Environment{{ID: "env-new", ProjectID: "one"}}); err != nil {
		t.Fatal(err)
	}
	items, err := repository.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].ID != "env-new" || items[1].ID != "env-two" {
		t.Fatalf("items = %#v", items)
	}
	updated := items[0]
	updated.Name = "updated"
	if err := repository.Update(context.Background(), updated); err != nil {
		t.Fatal(err)
	}
	stored, err := repository.Get(context.Background(), updated.ID)
	if err != nil || stored.Name != "updated" {
		t.Fatalf("Get()=%#v error=%v", stored, err)
	}
}

func TestMemoryRepositoryHonorsCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	repository := NewMemoryRepository()
	if _, err := repository.List(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("List() error = %v", err)
	}
	if err := repository.ReplaceProject(ctx, "project", nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("ReplaceProject() error = %v", err)
	}
}
