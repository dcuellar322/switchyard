package domain

import (
	"strings"
	"testing"
)

func TestCreateRequestValidatesTypedLaunchKinds(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		request CreateRequest
		valid   bool
	}{
		{name: "shell", request: CreateRequest{ProjectID: "project_one", Kind: KindShell, Shell: "zsh", Columns: 120, Rows: 36}, valid: true},
		{name: "service", request: CreateRequest{ProjectID: "project_one", Kind: KindService, ServiceID: "api", Shell: "sh", Columns: 80, Rows: 24}, valid: true},
		{name: "database", request: CreateRequest{ProjectID: "project_one", Kind: KindDatabase, ServiceID: "db", DatabaseClient: "psql", Columns: 80, Rows: 24}, valid: true},
		{name: "agent", request: CreateRequest{ProjectID: "project_one", Kind: KindAgent, Provider: "codex", Columns: 80, Rows: 24}, valid: true},
		{name: "action", request: CreateRequest{ProjectID: "project_one", Kind: KindAction, ActionID: "console", Columns: 80, Rows: 24}, valid: true},
		{name: "raw command has no surface", request: CreateRequest{ProjectID: "project_one", Kind: Kind("command"), Columns: 80, Rows: 24}, valid: false},
		{name: "invalid dimensions", request: CreateRequest{ProjectID: "project_one", Kind: KindShell, Columns: 501, Rows: 24}, valid: false},
		{name: "shell cross kind fields", request: CreateRequest{ProjectID: "project_one", Kind: KindShell, Provider: "codex", Columns: 80, Rows: 24}, valid: false},
		{name: "unknown database client", request: CreateRequest{ProjectID: "project_one", Kind: KindDatabase, ServiceID: "db", DatabaseClient: "shell", Columns: 80, Rows: 24}, valid: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.request.Validate()
			if test.valid && err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
			if !test.valid && err == nil {
				t.Fatal("Validate() expected error")
			}
		})
	}
}

func TestOwnerRejectsUnboundedOrUntrustedIdentityText(t *testing.T) {
	t.Parallel()
	for _, owner := range []Owner{{Type: "", ID: "browser"}, {Type: "browser", ID: strings.Repeat("x", 129)}, {Type: "browser cookie", ID: "one"}} {
		if err := owner.Validate(); err == nil {
			t.Fatalf("Validate(%+v) expected error", owner)
		}
	}
	if err := (Owner{Type: "browser", ID: "session_123"}).Validate(); err != nil {
		t.Fatalf("valid owner rejected: %v", err)
	}
}
