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
	RepeatContext(ctx context.Context, repeatFunc func() bool) (success bool)
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
		time.Sleep(r.sleeper.SleepTime(i))
		needRepeat = repeatFunc()

		if !needRepeat {
			return true
		}
	}

	return false
}

func (r *repeater) RepeatContext(ctx context.Context, repeatFunc func() bool) (success bool) {
	needRepeat := repeatFunc()
	if !needRepeat {
		return true
	}

	repeatCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	execCh := make(chan struct{})
	defer close(execCh)

	ticker := r.tickerContext(repeatCtx, execCh)

	for range ticker {
		needRepeat := repeatFunc()
		if !needRepeat {
			return true
		}

		execCh <- struct{}{}
	}

	return false
}

func (r *repeater) tickerContext(ctx context.Context, ch chan struct{}) <-chan struct{} {
	ticker := make(chan struct{})

	go func() {
		defer close(ticker)

		for repeatCount := 0; repeatCount < r.repeatTimes; repeatCount++ {
			sleepTime := r.sleeper.SleepTime(repeatCount)
			timer := time.NewTimer(sleepTime)

			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				ticker <- struct{}{}
			}

			<-ch
		}
	}()

	return ticker
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

func (p StandardSleeper) SleepTime(repeatCount int) time.Duration {
	return time.Duration(p)
}
