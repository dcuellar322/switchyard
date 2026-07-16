package adapters

import (
	"testing"
	"time"
)

func TestParseListenerName(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		input string
		host  string
		port  int
	}{
		{"TCP *:8080", "0.0.0.0", 8080},
		{"TCP 127.0.0.1:15173", "127.0.0.1", 15173},
		{"TCP [::1]:3000", "::1", 3000},
	} {
		host, port, ok := parseListenerName(test.input)
		if !ok || host != test.host || port != test.port {
			t.Fatalf("parse %q = %q %d %v", test.input, host, port, ok)
		}
	}
}

func TestOSListenersDeduplicatesRepeatedSocketRows(t *testing.T) {
	t.Parallel()
	listeners := &OSListeners{now: time.Now}
	facts := listeners.parse([]byte("p42\ncserver\nn*:8080\nn*:8080\n"), "tcp")
	if len(facts) != 1 {
		t.Fatalf("facts = %#v", facts)
	}
}
