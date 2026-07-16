package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestHumanAndMachineOutputGoldens(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, golden string
		options      rootOptions
		data         any
		human        func(*bytes.Buffer) error
	}{
		{name: "human", golden: "project-list-human.golden", data: []string{"alpha", "beta"}, human: func(buffer *bytes.Buffer) error {
			return humanList(buffer, []string{"PROJECT", "TRUST"}, [][]string{{"alpha", "trusted"}, {"beta", "pending"}})
		}},
		{name: "machine", golden: "project-list-json.golden", options: rootOptions{json: true}, data: []map[string]any{{"slug": "alpha", "trustState": "trusted"}}, human: func(*bytes.Buffer) error { return nil }},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			test.options.stdout = &output
			if err := writeResult(&test.options, "project.list", test.data, func(_ io.Writer) error { return test.human(&output) }); err != nil {
				t.Fatal(err)
			}
			want, err := os.ReadFile(filepath.Join("testdata", test.golden))
			if err != nil {
				t.Fatal(err)
			}
			if output.String() != string(want) {
				t.Fatalf("output:\n%s\nwant:\n%s", output.String(), want)
			}
		})
	}
}
