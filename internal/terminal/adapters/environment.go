package adapters

import (
	"os"
	"sort"
	"strings"
)

func terminalEnvironment(overlay map[string]string) []string {
	values := make(map[string]string)
	for _, entry := range os.Environ() {
		key, value, found := strings.Cut(entry, "=")
		if found {
			values[key] = value
		}
	}
	values["TERM"] = "xterm-256color"
	values["COLORTERM"] = "truecolor"
	for key, value := range overlay {
		values[key] = value
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]string, 0, len(keys))
	for _, key := range keys {
		result = append(result, key+"="+values[key])
	}
	return result
}
