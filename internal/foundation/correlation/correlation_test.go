package correlation

import (
	"context"
	"testing"
)

func TestIDRoundTrip(t *testing.T) {
	t.Parallel()

	id, err := NewID()
	if err != nil {
		t.Fatalf("NewID() error = %v", err)
	}
	if len(id) != 32 {
		t.Fatalf("NewID() length = %d, want 32", len(id))
	}
	if got := ID(WithID(context.Background(), id)); got != id {
		t.Fatalf("ID() = %q, want %q", got, id)
	}
}
