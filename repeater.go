package repeater

import (
	"context"
	"time"
)

type DurationProgression interface {
	Duration(times uint64) time.Duration
}

type Repeater interface {
	Repeat(f func() (ok bool), count uint64) (success bool)
	RepeatContext(ctx context.Context, f func(ctx context.Context) (ok bool), count uint64) (success bool)
}

func New(progression DurationProgression) Repeater {
	return &repeater{progression: progression}
}

type repeater struct {
	progression DurationProgression
}

func (r *repeater) Repeat(f func() bool, count uint64) (success bool) {
	ok := f()
	if ok {
		return true
	}

	for i := range count {
		sleepTime := r.progression.Duration(i)
		if sleepTime <= 0 {
			ok := f()
			if ok {
				return true
			}

			continue
		}

		<-time.After(sleepTime)

		ok := f()
		if ok {
			return true
		}
	}

	return false
}

func (r *repeater) RepeatContext(ctx context.Context, f func(ctx context.Context) bool, count uint64) (success bool) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	ok := f(ctx)
	if ok {
		return true
	}

	for i := range count {
		sleepTime := r.progression.Duration(i)
		if sleepTime <= 0 {
			ok := f(ctx)
			if ok {
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
			ok := f(ctx)
			if ok {
				return true
			}
		}
	}

	return false
}

type ArifmeticProggression struct {
	initial time.Duration
	delta   time.Duration
}

func NewArifmeticProgression(initial, delta time.Duration) ArifmeticProggression {
	return ArifmeticProggression{initial: initial, delta: delta}
}

func (a ArifmeticProggression) Duration(times uint64) time.Duration {
	return a.initial + (a.delta * time.Duration(times))
}

type ConstantProgression time.Duration

func (p ConstantProgression) Duration(uint64) time.Duration {
	return time.Duration(p)
}

type FibonacciProgression time.Duration

func (s FibonacciProgression) Duration(repeatCount uint64) time.Duration {
	return time.Duration(s) * time.Duration(fibonacciIterative(repeatCount+1))
}

func fibonacciIterative(n uint64) uint64 {
	if n <= 1 {
		return n
	}

	var n2, n1 uint64 = 0, 1
	for i := uint64(2); i <= n; i++ {
		n2, n1 = n1, n1+n2
	}

	return n1
}
