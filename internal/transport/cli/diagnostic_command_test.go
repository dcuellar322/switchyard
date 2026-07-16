package cli

import (
	"bytes"
	"errors"
	"testing"
)

func TestDiagnosticAndAutomationCommandsRequireExplicitReview(t *testing.T) {
	t.Parallel()
	tests := []struct {
		args []string
		code string
	}{
		{args: []string{"diagnose", "run", "diagnosis-1", "tests"}, code: "DIAGNOSTIC_ACTION_CONFIRMATION_REQUIRED"},
		{args: []string{"diagnose", "feedback", "diagnosis-1", "finding-1", "--verdict", "maybe"}, code: "DIAGNOSTIC_FEEDBACK_INVALID"},
		{args: []string{"automation", "enable", "recipe-1"}, code: "AUTOMATION_CONFIRMATION_REQUIRED"},
	}
	for _, test := range tests {
		command := newRootCommand(&rootOptions{stdout: &bytes.Buffer{}, stderr: &bytes.Buffer{}})
		command.SetArgs(test.args)
		err := command.Execute()
		var cliError *Error
		if !errors.As(err, &cliError) || cliError.Code != test.code {
			t.Errorf("args=%v error=%#v", test.args, err)
		}
	}
}
