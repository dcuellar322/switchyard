package process

import (
	"math"
	"testing"
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
