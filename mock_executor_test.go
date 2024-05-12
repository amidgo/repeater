package repeater_test

import (
	"context"
	"testing"
	"time"
)

type MockExecutor struct {
	testing              *testing.T
	expectedExecuteCount int
	executeDeadline      <-chan time.Time
}

func NewMockExecutor(t *testing.T, expectedExecuteCount int, executeDeadline time.Duration) *MockExecutor {
	deadlineCh := time.After(executeDeadline)

	md := &MockExecutor{
		testing:              t,
		expectedExecuteCount: expectedExecuteCount,
		executeDeadline:      deadlineCh,
	}

	md.SetCleanupFunc()

	return md
}

func (d *MockExecutor) SetCleanupFunc() {
	d.testing.Cleanup(func() {
		select {
		case <-time.After(time.Millisecond * 100):
			d.testing.Fatal("wrong deadline time")
		case <-d.executeDeadline:
		}

		if d.expectedExecuteCount != 0 {
			d.testing.Fatalf("wrong execute count, %d execute left", d.expectedExecuteCount)
		}
	})
}

func (d *MockExecutor) Execute() bool {
	select {
	case <-d.executeDeadline:
		d.testing.Fatal("deadline time is over")

		return false
	default:
		d.expectedExecuteCount--
		return d.expectedExecuteCount > 0
	}
}

func (d *MockExecutor) ExecuteContext(context.Context) bool {
	select {
	case <-d.executeDeadline:
		d.testing.Fatal("deadline time is over")

		return false
	default:
		d.expectedExecuteCount--
		return d.expectedExecuteCount > 0
	}
}
