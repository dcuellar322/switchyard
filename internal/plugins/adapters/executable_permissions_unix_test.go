//go:build !windows

package adapters

import (
	"os"
	"testing"
)

func TestValidateExecutablePermissions(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name    string
		mode    os.FileMode
		wantErr bool
	}{
		{name: "owner executable", mode: 0o700},
		{name: "not executable", mode: 0o600, wantErr: true},
		{name: "group writable", mode: 0o720, wantErr: true},
		{name: "world writable", mode: 0o702, wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := validateExecutablePermissions(test.mode); (err != nil) != test.wantErr {
				t.Fatalf("validateExecutablePermissions(%#o) error = %v, wantErr %t", test.mode, err, test.wantErr)
			}
		})
	}
}
