package cli

import (
	"net/url"
	"strings"
)

func validateUIPath(value string) (string, error) {
	if value == "" {
		value = "/"
	}
	if strings.Contains(value, "#") {
		return "", usageError("UI_PATH_INVALID", "--path cannot contain fragments, dot segments, backslashes, or bootstrap credentials")
	}
	parsed, err := url.ParseRequestURI(value)
	if err != nil || parsed.IsAbs() || parsed.Host != "" || !strings.HasPrefix(parsed.Path, "/") || strings.HasPrefix(parsed.Path, "//") {
		return "", usageError("UI_PATH_INVALID", "--path must be one local application route beginning with /")
	}
	if parsed.Fragment != "" || strings.Contains(parsed.Path, `\`) || hasDotPathSegment(parsed.EscapedPath()) || parsed.Query().Has("bootstrap") {
		return "", usageError("UI_PATH_INVALID", "--path cannot contain fragments, dot segments, backslashes, or bootstrap credentials")
	}
	return parsed.String(), nil
}

func hasDotPathSegment(path string) bool {
	for _, segment := range strings.Split(strings.ToLower(path), "/") {
		if segment == "." || segment == ".." || segment == "%2e" || segment == "%2e%2e" || segment == ".%2e" || segment == "%2e." {
			return true
		}
	}
	return false
}
