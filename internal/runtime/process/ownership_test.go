package process

import (
	"context"
	"math"
	"testing"
	"time"

	"switchyard.dev/switchyard/internal/runtime/domain"
)

func TestBoundedPIDRejectsInvalidNativeValues(t *testing.T) {
	t.Parallel()
	for _, value := range []int{0, -1} {
		if _, err := boundedPID(value); err == nil {
			t.Fatalf("boundedPID(%d) accepted an invalid PID", value)
		}
	}
	if int64(math.MaxInt) > math.MaxInt32 {
		if _, err := boundedPID(math.MaxInt); err == nil {
			t.Fatal("boundedPID(MaxInt) accepted a PID wider than the durable format")
		}
	}
	if got, err := boundedPID(42); err != nil || got != 42 {
		t.Fatalf("boundedPID(42) = %d, %v", got, err)
	}
}

func TestGroupMembersPrefersOwnedProcessMembership(t *testing.T) {
	t.Parallel()
	startedAt := time.Now().UTC()
	inspector := inspectorFake{
		snapshots: map[int32]domain.ProcessIdentity{
			10: {PID: 10, ProcessGroup: 10, StartedAt: startedAt, Fingerprint: "parent"},
			11: {PID: 11, ProcessGroup: 11, StartedAt: startedAt, Fingerprint: "child"},
		},
		groups: map[int32][]domain.ProcessIdentity{
			10: {{PID: 99, ProcessGroup: 10, Fingerprint: "fallback"}},
		},
	}
	driver := newDriver(context.Background(), newMemoryRunStore(), inspector, &secretResolverFake{})
	ownership := &memberSourceOwnershipFake{group: 10, pids: []int32{10, 11}}

	members, err := driver.groupMembers(context.Background(), ownership, ownership.group)
	if err != nil {
		t.Fatal(err)
	}
	if ownership.calls != 1 || len(members) != 2 {
		t.Fatalf("membership calls = %d, members = %#v", ownership.calls, members)
	}
	for _, member := range members {
		if member.ProcessGroup != ownership.group || member.PID == 99 {
			t.Fatalf("member = %#v", member)
		}
	}
}

type memberSourceOwnershipFake struct {
	group int32
	pids  []int32
	calls int
}

func (o *memberSourceOwnershipFake) Group() int32    { return o.group }
func (*memberSourceOwnershipFake) Running() bool     { return true }
func (*memberSourceOwnershipFake) Signal(bool) error { return nil }
func (*memberSourceOwnershipFake) Close() error      { return nil }
func (o *memberSourceOwnershipFake) MemberPIDs() ([]int32, error) {
	o.calls++
	return append([]int32(nil), o.pids...), nil
}
