package adapters

import (
	"context"
	"syscall"
	"testing"
	"time"

	gnet "github.com/shirou/gopsutil/v4/net"
)

func TestOSListenersObservesPortableTCPAndUDPBindings(t *testing.T) {
	t.Parallel()
	observed := time.Date(2026, time.July, 16, 12, 0, 0, 0, time.UTC)
	listeners := &OSListeners{
		connections: func(context.Context, string) ([]gnet.ConnectionStat, error) {
			return []gnet.ConnectionStat{
				{Type: syscall.SOCK_STREAM, Status: "LISTEN", Laddr: gnet.Addr{IP: "::", Port: 8080}, Pid: 42},
				{Type: syscall.SOCK_STREAM, Status: "ESTABLISHED", Laddr: gnet.Addr{IP: "127.0.0.1", Port: 8081}, Pid: 42},
				{Type: syscall.SOCK_DGRAM, Laddr: gnet.Addr{IP: "127.0.0.1", Port: 5353}, Pid: 9},
				{Type: syscall.SOCK_STREAM, Status: "LISTEN", Laddr: gnet.Addr{IP: "::", Port: 8080}, Pid: 42},
			}, nil
		},
		processName: func(_ context.Context, pid int32) string {
			if pid == 42 {
				return "server"
			}
			return "resolver"
		},
		now: func() time.Time { return observed },
	}
	facts, err := listeners.Facts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(facts) != 2 {
		t.Fatalf("facts = %#v", facts)
	}
	if facts[0].Port != 5353 || facts[0].Protocol != "udp" || facts[1].Port != 8080 || facts[1].Host != "0.0.0.0" {
		t.Fatalf("facts = %#v", facts)
	}
	if facts[1].Evidence != "live listener owned by server (pid 42)" || !facts[1].ObservedAt.Equal(observed) {
		t.Fatalf("tcp fact = %#v", facts[1])
	}
}
