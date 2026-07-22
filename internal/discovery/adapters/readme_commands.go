package adapters

import (
	"regexp"
	"strconv"
	"strings"

	"switchyard.dev/switchyard/internal/discovery/domain"
)

var (
	pythonApplication = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.]*:[A-Za-z_][A-Za-z0-9_]*$`)
	uvicornLogLevel   = regexp.MustCompile(`^(critical|error|warning|info|debug|trace)$`)
)

// scanDocumentedCommands recognizes only a narrow shell-free command grammar
// inside fenced README examples. Repository prose remains untrusted and no
// command is executed during discovery.
func scanDocumentedCommands(path string, contents []byte) ([]domain.Evidence, error) {
	lines := strings.Split(string(contents), "\n")
	inShellFence := false
	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			if inShellFence {
				inShellFence = false
				continue
			}
			language := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(trimmed, "```")))
			inShellFence = language == "bash" || language == "sh" || language == "shell" || language == "zsh" || language == "console"
			continue
		}
		if !inShellFence {
			continue
		}
		command, port, ok := documentedUvicornCommand(strings.TrimPrefix(trimmed, "$ "))
		if !ok {
			continue
		}
		return evidence("python.run", path, index+1, index+1, .86, map[string]any{"command": command, "port": port})
	}
	return nil, nil
}

func documentedUvicornCommand(line string) ([]string, int, bool) {
	if line == "" || strings.ContainsAny(line, ";&|`$<>") {
		return nil, 0, false
	}
	fields := strings.Fields(line)
	targetIndex, ok := uvicornTargetIndex(fields)
	if !ok || !pythonApplication.MatchString(fields[targetIndex]) {
		return nil, 0, false
	}
	port, ok := documentedUvicornOptions(fields, targetIndex+1)
	if !ok {
		return nil, 0, false
	}
	return fields, port, true
}

func uvicornTargetIndex(fields []string) (int, bool) {
	switch {
	case len(fields) >= 4 && fields[0] == "uv" && fields[1] == "run" && fields[2] == "uvicorn":
		return 3, true
	case len(fields) >= 6 && fields[0] == "uv" && fields[1] == "run" && fields[2] == "python" && fields[3] == "-m" && fields[4] == "uvicorn":
		return 5, true
	case len(fields) >= 4 && fields[0] == "python" && fields[1] == "-m" && fields[2] == "uvicorn":
		return 3, true
	default:
		return 0, false
	}
}

func documentedUvicornOptions(fields []string, start int) (int, bool) {
	port := 8000
	for index := start; index < len(fields); index++ {
		field := fields[index]
		switch {
		case field == "--reload" || field == "--access-log" || field == "--no-access-log":
			continue
		case field == "--host" && index+1 < len(fields):
			index++
			if !safeUvicornHost(fields[index]) {
				return 0, false
			}
		case strings.HasPrefix(field, "--host="):
			if !safeUvicornHost(strings.TrimPrefix(field, "--host=")) {
				return 0, false
			}
		case field == "--port" && index+1 < len(fields):
			index++
			parsed, ok := parseDocumentedPort(fields[index])
			if !ok {
				return 0, false
			}
			port = parsed
		case strings.HasPrefix(field, "--port="):
			parsed, ok := parseDocumentedPort(strings.TrimPrefix(field, "--port="))
			if !ok {
				return 0, false
			}
			port = parsed
		case field == "--log-level" && index+1 < len(fields):
			index++
			if !uvicornLogLevel.MatchString(fields[index]) {
				return 0, false
			}
		default:
			return 0, false
		}
	}
	return port, true
}

func safeUvicornHost(value string) bool {
	return value == "127.0.0.1" || value == "localhost" || value == "0.0.0.0" || value == "::1"
}

func parseDocumentedPort(value string) (int, bool) {
	parsed, err := strconv.Atoi(value)
	return parsed, err == nil && parsed >= 1 && parsed <= 65535
}
