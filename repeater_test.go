package repeater_test

import (
	"context"
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/amidgo/repeater"
	"github.com/amidgo/tester"
	"github.com/stretchr/testify/assert"
)

func Test_Repeater_Full_Repeat(t *testing.T) {
	t.Parallel()

	tester.RunNamedTesters(t,
		RepeaterCase{
			RepeatCount: 2,
			RepeatPause: time.Second,
		},
		RepeaterCase{
			RepeatCount: 1,
			RepeatPause: time.Second,
		},
		RepeaterCase{
			RepeatCount: 10,
			RepeatPause: time.Millisecond * 500,
		},
		RepeaterCase{
			RepeatCount: 1,
			RepeatPause: time.Millisecond * 500,
		},
		RepeaterCase{
			RepeatCount: -1,
			RepeatPause: time.Second,
		},
	)

	t.Log(runtime.NumGoroutine())
}

type RepeaterCase struct {
	RepeatCount int
	RepeatPause time.Duration
}

func (c RepeaterCase) Name() string {
	return fmt.Sprintf("repeater test count %d, repeat pause %s", c.RepeatCount, c.RepeatPause)
}

func (c RepeaterCase) Test(t *testing.T) {
	t.Parallel()
	t.Run("repeat", c.testRepeat)
	t.Run("context repeat", c.testRepeatContext)
}

func (c *RepeaterCase) testRepeat(t *testing.T) {
	t.Parallel()
	executor := c.MockExecutor(t)
	repeater := c.Repeater()

	success := repeater.Repeat(executor.Execute)

	assert.True(t, success)
}

func (c *RepeaterCase) testRepeatContext(t *testing.T) {
	t.Parallel()
	executor := c.MockExecutor(t)
	repeater := c.Repeater()

	ctx, cancel := context.WithTimeout(context.Background(), c.RepeatDeadline())
	defer cancel()

	success := repeater.RepeatContext(ctx, executor.ExecuteContext)

	assert.True(t, success)
}

func (c *RepeaterCase) MockExecutor(t *testing.T) *MockExecutor {
	md := NewMockExecutor(t, c.ExecuteCount(), c.RepeatDeadline())

	return md
}

func (c *RepeaterCase) ExecuteCount() int {
	return 1 + max(0, c.RepeatCount)
}

func (c *RepeaterCase) RepeatDeadline() time.Duration {
	return c.AddedTime() + c.RepeatedExecuteDuration()
}

func (c *RepeaterCase) AddedTime() time.Duration {
	return time.Millisecond * 90
}

func (c *RepeaterCase) RepeatedExecuteDuration() time.Duration {
	repeatedExecuteDuration := time.Duration(c.RepeatCount) * c.RepeatPause
	if repeatedExecuteDuration < 0 {
		return 0
	}

	return repeatedExecuteDuration
}

func (c *RepeaterCase) Repeater() repeater.Repeater {
	return repeater.NewRepeater(c.RepeatCount, repeater.StandardSleeper(c.RepeatPause))
}

func Test_Repeater_False(t *testing.T) {
	t.Parallel()
	sleeper := repeater.NewPauseSleeper()

	repeater := repeater.NewRepeater(10, sleeper)

	res := repeater.Repeat(func() bool { return true })
	assert.False(t, res)
}

func Test_Repeater_Context_False(t *testing.T) {
	t.Parallel()

	tester.RunNamedTesters(t,
		&RepeaterContextCase{
			Sleeper:              repeater.StandardSleeper(time.Second),
			RepeatCount:          5,
			ContextTimeout:       time.Millisecond * 4500,
			RepeatFunc:           func(context.Context, int) bool { return true },
			ExpectedExecuteCount: 5,
			ExpectedResult:       false,
		},
		&RepeaterContextCase{
			Sleeper:        repeater.StandardSleeper(time.Second),
			RepeatCount:    5,
			ContextTimeout: time.Millisecond * 4500,
			RepeatFunc: func(context.Context, int) bool {
				<-time.After(time.Second)

				return true
			},
			ExpectedExecuteCount: 3,
			ExpectedResult:       false,
		},
		&RepeaterContextCase{
			Sleeper:              repeater.StandardSleeper(time.Second),
			RepeatCount:          5,
			ContextTimeout:       time.Millisecond * 5500,
			RepeatFunc:           func(_ context.Context, count int) bool { return count < 5 },
			ExpectedExecuteCount: 6,
			ExpectedResult:       true,
		},
	)
}

type RepeaterContextCase struct {
	Sleeper              repeater.Sleeper
	RepeatCount          int
	ContextTimeout       time.Duration
	RepeatFunc           func(ctx context.Context, count int) bool
	ExpectedExecuteCount int
	ExpectedResult       bool
}

func (c *RepeaterContextCase) Name() string {
	return fmt.Sprintf(
		"repeat count %d, context timeout %s, expected execute count %d, expected result %t",
		c.RepeatCount, c.ContextTimeout, c.ExpectedExecuteCount, c.ExpectedResult,
	)
}

func (c *RepeaterContextCase) Test(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), c.ContextTimeout)
	defer cancel()

	repeater := repeater.NewRepeater(c.RepeatCount, c.Sleeper)

	var count int
	res := repeater.RepeatContext(ctx,
		func(ctx context.Context) bool {
			deadline, _ := ctx.Deadline()
			t.Logf("count %d, deadline %s", count, time.Until(deadline))
			res := c.RepeatFunc(ctx, count)
			count++

			return res
		},
	)

	assert.Equal(t, c.ExpectedResult, res)
	assert.Equal(t, c.ExpectedExecuteCount, count)
}

/*
	repeat cycle
	first do - 1 seconds
	sleep(1s) - 2 seconds
	second do - 3 seconds
	sleep(1s) - 4 seconds
	third do  - 5 seconds

	sleep(1s) - 3 seconds
	fourth do - 3 seconds
	sleep(1s) - 4 seconds
	fifth do  - 4 seconds
	sleep(1s) - 5 seconds
	sixth do  - 5 seconds
	context canceled
	return false
*/
