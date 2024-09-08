package repeater_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/amidgo/repeater"
	"github.com/amidgo/tester"
	"github.com/stretchr/testify/assert"
)

func Test_ConstantProgression(t *testing.T) {
	tester.RunNamedTesters(t,
		NewSleeperTestСase(repeater.ConstantProgression(time.Second), 132, time.Second),
		NewSleeperTestСase(repeater.ConstantProgression(time.Second), 1, time.Second),
		NewSleeperTestСase(repeater.ConstantProgression(time.Second*10), 132, time.Second*10),
	)
}

func Test_ArifmeticProgression(t *testing.T) {
	tester.RunNamedTesters(t,
		NewSleeperTestСase(repeater.NewArifmeticProgression(time.Second, time.Second*2), 1, time.Second*3),
		NewSleeperTestСase(repeater.NewArifmeticProgression(time.Second, time.Second*2), 0, time.Second),
		NewSleeperTestСase(repeater.NewArifmeticProgression(time.Second, time.Second*2), 3, time.Second*7),
	)
}

func Test_FibanacciProgression(t *testing.T) {
	tester.RunNamedTesters(t,
		NewSleeperTestСase(repeater.FibonacciProgression(time.Second), 0, time.Second),
		NewSleeperTestСase(repeater.FibonacciProgression(time.Second), 1, time.Second),
		NewSleeperTestСase(repeater.FibonacciProgression(time.Second), 2, time.Second*2),
		NewSleeperTestСase(repeater.FibonacciProgression(time.Second), 3, time.Second*3),
		NewSleeperTestСase(repeater.FibonacciProgression(time.Second), 4, time.Second*5),
		NewSleeperTestСase(repeater.FibonacciProgression(time.Second), 5, time.Second*8),
		NewSleeperTestСase(repeater.FibonacciProgression(time.Second), 6, time.Second*13),
		NewSleeperTestСase(repeater.FibonacciProgression(time.Second), 7, time.Second*21),
		NewSleeperTestСase(repeater.FibonacciProgression(time.Second), 8, time.Second*34),
		NewSleeperTestСase(repeater.FibonacciProgression(time.Second), 9, time.Second*55),
	)
}

type SleeperTestCase struct {
	Sleeper           repeater.DurationProgression
	RepeatCount       uint64
	ExpectedSleepTime time.Duration
}

func NewSleeperTestСase(
	sleeper repeater.DurationProgression,
	repeatCount uint64,
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
	sleepTime := s.Sleeper.Duration(s.RepeatCount)

	assert.Equal(t, s.ExpectedSleepTime, sleepTime)
}
