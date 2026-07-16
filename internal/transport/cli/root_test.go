package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestVersionCommand(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	if err := Execute(context.Background(), []string{"version"}, &output, &output); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.HasPrefix(output.String(), "Switchyard dev") {
		t.Fatalf("output = %q", output.String())
	}
}
