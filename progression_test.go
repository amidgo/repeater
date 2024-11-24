package repeater_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/amidgo/repeater"
	"github.com/amidgo/tester"
)

type ProgressionTest struct {
	Progression      repeater.DurationProgression
	Time             uint64
	ExpectedDuration time.Duration
}

func (s *ProgressionTest) Name() string {
	return fmt.Sprintf("repeat count %d expected sleep time %s", s.Time, s.ExpectedDuration)
}

func (s *ProgressionTest) Test(t *testing.T) {
	sleepTime := s.Progression.Duration(s.Time)

	if s.ExpectedDuration != sleepTime {
		t.Fatalf("wrong duration, expected %s, actual %s", s.ExpectedDuration, sleepTime)
	}
}

func Test_ConstantProgression(t *testing.T) {
	tester.RunNamedTesters(t,
		&ProgressionTest{
			Progression:      repeater.ConstantProgression(time.Second),
			Time:             132,
			ExpectedDuration: time.Second,
		},
		&ProgressionTest{
			Progression:      repeater.ConstantProgression(time.Second),
			Time:             1,
			ExpectedDuration: time.Second,
		},
		&ProgressionTest{
			Progression:      repeater.ConstantProgression(time.Second * 10),
			Time:             1,
			ExpectedDuration: time.Second * 10,
		},
	)
}

func Test_ArifmeticProgression(t *testing.T) {
	tester.RunNamedTesters(t,
		&ProgressionTest{
			Progression:      repeater.NewArifmeticProgression(time.Second, time.Second*2),
			Time:             1,
			ExpectedDuration: time.Second * 3,
		},
		&ProgressionTest{
			Progression:      repeater.NewArifmeticProgression(time.Second, time.Second*2),
			Time:             0,
			ExpectedDuration: time.Second,
		},
		&ProgressionTest{
			Progression:      repeater.NewArifmeticProgression(time.Second, time.Second*2),
			Time:             3,
			ExpectedDuration: time.Second * 7,
		},
	)
}

func Test_FibanacciProgression(t *testing.T) {
	tester.RunNamedTesters(t,
		&ProgressionTest{
			Progression:      repeater.FibonacciProgression(time.Second),
			Time:             0,
			ExpectedDuration: time.Second,
		},
		&ProgressionTest{
			Progression:      repeater.FibonacciProgression(time.Second),
			Time:             1,
			ExpectedDuration: time.Second,
		},
		&ProgressionTest{
			Progression:      repeater.FibonacciProgression(time.Second),
			Time:             2,
			ExpectedDuration: time.Second * 2,
		},
		&ProgressionTest{
			Progression:      repeater.FibonacciProgression(time.Second),
			Time:             3,
			ExpectedDuration: time.Second * 3,
		},
		&ProgressionTest{
			Progression:      repeater.FibonacciProgression(time.Second),
			Time:             4,
			ExpectedDuration: time.Second * 5,
		},
		&ProgressionTest{
			Progression:      repeater.FibonacciProgression(time.Second),
			Time:             5,
			ExpectedDuration: time.Second * 8,
		},
		&ProgressionTest{
			Progression:      repeater.FibonacciProgression(time.Second),
			Time:             6,
			ExpectedDuration: time.Second * 13,
		},
		&ProgressionTest{
			Progression:      repeater.FibonacciProgression(time.Second),
			Time:             7,
			ExpectedDuration: time.Second * 21,
		},
		&ProgressionTest{
			Progression:      repeater.FibonacciProgression(time.Second),
			Time:             8,
			ExpectedDuration: time.Second * 34,
		},
		&ProgressionTest{
			Progression:      repeater.FibonacciProgression(time.Second),
			Time:             9,
			ExpectedDuration: time.Second * 55,
		},
	)
}
