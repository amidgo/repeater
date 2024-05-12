package repeater

import (
	"context"
	"time"
)

type Sleeper interface {
	SleepTime(repeatCount int) time.Duration
}

/*
repeatFunc is a function which returns true if needRepeat and false if done,
Repeat and RepeatContext functions returns true if one of calls repeatFunc returns true
Repeat return false if repeat count exceeded
RepeatContext return false if repeat count exceeded or context done
*/
type Repeater interface {
	Repeat(repeatFunc func() bool) (success bool)
	RepeatContext(ctx context.Context, repeatFunc func(ctx context.Context) bool) (success bool)
}

func NewRepeater(n int, sleeper Sleeper) Repeater {
	return &repeater{repeatTimes: n, sleeper: sleeper}
}

type repeater struct {
	repeatTimes int
	sleeper     Sleeper
}

func (r *repeater) Repeat(repeatFunc func() bool) (success bool) {
	needRepeat := repeatFunc()
	if !needRepeat {
		return true
	}

	for i := 0; i < r.repeatTimes; i++ {
		<-time.After(r.sleeper.SleepTime(i))
		needRepeat = repeatFunc()

		if !needRepeat {
			return true
		}
	}

	return false
}

func (r *repeater) RepeatContext(ctx context.Context, repeatFunc func(ctx context.Context) bool) (success bool) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	needRepeat := repeatFunc(ctx)
	if !needRepeat {
		return true
	}

	for i := range r.repeatTimes {
		sleepTime := r.sleeper.SleepTime(i)
		if sleepTime <= 0 {
			needRepeat := repeatFunc(ctx)
			if !needRepeat {
				return true
			}

			continue
		}

		timer := time.NewTimer(sleepTime)

		select {
		case <-ctx.Done():
			timer.Stop()

			return false
		case <-timer.C:
			needRepeat := repeatFunc(ctx)
			if !needRepeat {
				return true
			}
		}
	}

	return false
}

type PauseSleeper []time.Duration

func NewPauseSleeper(pauses ...time.Duration) PauseSleeper {
	return PauseSleeper(pauses)
}

func (p PauseSleeper) SleepTime(repeatCount int) time.Duration {
	if repeatCount > len(p)-1 {
		return 0
	}

	return p[repeatCount]
}

type StandardSleeper time.Duration

func (p StandardSleeper) SleepTime(int) time.Duration {
	return time.Duration(p)
}

type FibonacciSleeper time.Duration

func (s FibonacciSleeper) SleepTime(repeatCount int) time.Duration {
	return time.Duration(s) * time.Duration(fibonacciIterative(repeatCount+1))
}

func fibonacciIterative(n int) int {
	if n <= 1 {
		return n
	}

	var n2, n1 = 0, 1
	for i := 2; i <= n; i++ {
		n2, n1 = n1, n1+n2
	}

	return n1
}

// executed: 1 1 1
// sleeped:  111111
