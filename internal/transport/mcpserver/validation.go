package mcpserver

import (
	"errors"
	"fmt"
	"strings"
)

func required(value, field string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s is required", field)
	}
	return nil
}

func requestID(value string) error {
	if length := len(value); length < 8 || length > 128 {
		return errors.New("requestId must contain between 8 and 128 characters")
	}
	return nil
}

func bounded(value, fallback, maximum int, field string) (int, error) {
	if value == 0 {
		return fallback, nil
	}
	if value < 1 || value > maximum {
		return 0, fmt.Errorf("%s must be between 1 and %d", field, maximum)
	}
	return value, nil
}
