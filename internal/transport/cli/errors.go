package cli

import (
	"errors"
	"fmt"
	"strings"

	"switchyard.dev/switchyard/internal/transport/httpclient"
)

const (
	exitUsage       = 2
	exitNotFound    = 3
	exitConflict    = 4
	exitUnavailable = 5
	exitInternal    = 6
)

// Error is the stable CLI failure contract.
type Error struct {
	Code     string
	Message  string
	ExitCode int
	Cause    error
}

func (e *Error) Error() string { return e.Message }
func (e *Error) Unwrap() error { return e.Cause }

func usageError(code, message string) error {
	return &Error{Code: code, Message: message, ExitCode: exitUsage}
}
func notFoundError(message string) error {
	return &Error{Code: "PROJECT_NOT_FOUND", Message: message, ExitCode: exitNotFound}
}
func conflictError(code, message string) error {
	return &Error{Code: code, Message: message, ExitCode: exitConflict}
}

func classifyError(err error) *Error {
	var cliErr *Error
	if errors.As(err, &cliErr) {
		return cliErr
	}
	var apiErr *httpclient.APIError
	if errors.As(err, &apiErr) {
		exitCode := exitInternal
		if apiErr.Status == 404 {
			exitCode = exitNotFound
		}
		if apiErr.Status == 409 || apiErr.Status == 422 {
			exitCode = exitConflict
		}
		if apiErr.Status == 403 {
			exitCode = exitConflict
		}
		if apiErr.Status == 503 {
			exitCode = exitUnavailable
		}
		code := apiErr.Code
		if code == "" {
			code = "API_ERROR"
		}
		return &Error{Code: code, Message: apiErr.Error(), ExitCode: exitCode, Cause: err}
	}
	message := err.Error()
	for _, fragment := range []string{"unknown command", "unknown flag", "accepts ", "requires at least", "required flag"} {
		if strings.Contains(message, fragment) {
			return &Error{Code: "CLI_USAGE", Message: message, ExitCode: exitUsage, Cause: err}
		}
	}
	for _, fragment := range []string{"dial unix", "connection refused", "no such file or directory", "context deadline exceeded"} {
		if strings.Contains(strings.ToLower(message), fragment) {
			return &Error{Code: "DAEMON_UNAVAILABLE", Message: message, ExitCode: exitUnavailable, Cause: err}
		}
	}
	return &Error{Code: "INTERNAL", Message: fmt.Sprint(err), ExitCode: exitInternal, Cause: err}
}
