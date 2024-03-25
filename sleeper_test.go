package repeater_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/amidgo/repeater"
	"github.com/amidgo/tester"
	"github.com/stretchr/testify/assert"
)

func Test_StandardSleeper(t *testing.T) {
	tester.RunNamedTesters(t,
		NewSleeperTestСase(repeater.StandardSleeper(time.Second), 132, time.Second),
		NewSleeperTestСase(repeater.StandardSleeper(time.Second), 1, time.Second),
		NewSleeperTestСase(repeater.StandardSleeper(time.Second*10), 132, time.Second*10),
	)
}

func Test_PauseSleeper(t *testing.T) {
	tester.RunNamedTesters(t,
		NewSleeperTestСase(repeater.NewPauseSleeper(time.Second, time.Second*2), 1, time.Second*2),
		NewSleeperTestСase(repeater.NewPauseSleeper(time.Second, time.Second*2), 0, time.Second),
		NewSleeperTestСase(repeater.NewPauseSleeper(time.Second, time.Second*2), 3, 0),
	)
}

func Test_FibanacciSleeper(t *testing.T) {
	tester.RunNamedTesters(t,
		NewSleeperTestСase(repeater.FibonacciSleeper(time.Second), 0, time.Second),
		NewSleeperTestСase(repeater.FibonacciSleeper(time.Second), 1, time.Second),
		NewSleeperTestСase(repeater.FibonacciSleeper(time.Second), 2, time.Second*2),
		NewSleeperTestСase(repeater.FibonacciSleeper(time.Second), 3, time.Second*3),
		NewSleeperTestСase(repeater.FibonacciSleeper(time.Second), 4, time.Second*5),
		NewSleeperTestСase(repeater.FibonacciSleeper(time.Second), 5, time.Second*8),
		NewSleeperTestСase(repeater.FibonacciSleeper(time.Second), 6, time.Second*13),
		NewSleeperTestСase(repeater.FibonacciSleeper(time.Second), 7, time.Second*21),
		NewSleeperTestСase(repeater.FibonacciSleeper(time.Second), 8, time.Second*34),
		NewSleeperTestСase(repeater.FibonacciSleeper(time.Second), 9, time.Second*55),
	)
}

type SleeperTestCase struct {
	Sleeper           repeater.Sleeper
	RepeatCount       int
	ExpectedSleepTime time.Duration
}

func NewSleeperTestСase(
	sleeper repeater.Sleeper,
	repeatCount int,
	expectedSleepTime time.Duration,
) SleeperTestCase {
	return SleeperTestCase{
		Sleeper:           sleeper,
		RepeatCount:       repeatCount,
		ExpectedSleepTime: expectedSleepTime,
	}
}

func (s SleeperTestCase) Name() string {
	return fmt.Sprintf("repeat count %d expected sleep time %s", s.RepeatCount, s.ExpectedSleepTime)
}

func (s SleeperTestCase) Test(t *testing.T) {
	sleepTime := s.Sleeper.SleepTime(s.RepeatCount)

	assert.Equal(t, s.ExpectedSleepTime, sleepTime)
}
