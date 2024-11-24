package repeater

import (
	"context"
	"time"
)

type (
	DurationProgression interface {
		// sleep duration by execute time
		// zero attempt is initial timeout
		// example:
		// ArifmeticProgression
		// initial: 1s delta: 0.5s
		// ArifmeticProgression.Duration(0) = 1s
		// ArifmeticProgression.Duration(1) = 1.5s
		// ArifmeticProgression.Duration(2) = 2s
		// ArifmeticProgression.Duration(3) = 2.5s
		Duration(attempt uint64) time.Duration
	}

	RepeatFunc        func() bool
	RepeatFuncContext func(ctx context.Context) bool
)

func Repeat(progression DurationProgression, rf RepeatFunc, retryCount uint64) (finished bool) {
	rp := New(progression)

	return rp.Repeat(rf, retryCount)
}

func RepeatContext(ctx context.Context, progresstion DurationProgression, rfctx RepeatFuncContext, retryCount uint64) (finished bool) {
	rp := New(progresstion)

	return rp.RepeatContext(ctx, rfctx, retryCount)
}

type Repeater struct {
	progression DurationProgression
}

func New(progression DurationProgression) *Repeater {
	return &Repeater{progression: progression}
}

func (r *Repeater) Repeat(rf RepeatFunc, retryCount uint64) (finished bool) {
	finished = rf()
	if finished {
		return true
	}

	for attempt := range retryCount {
		sleepTime := r.progression.Duration(attempt)
		if sleepTime <= 0 {
			finished = rf()
			if finished {
				return true
			}

			continue
		}

		<-time.After(sleepTime)

		finished = rf()
		if finished {
			return true
		}
	}

	return false
}

func (r *Repeater) RepeatContext(ctx context.Context, rfctx RepeatFuncContext, retryCount uint64) (finished bool) {
	finished = rfctx(ctx)
	if finished {
		return true
	}

	for attempt := range retryCount {
		sleepTime := r.progression.Duration(attempt)
		if sleepTime <= 0 {
			finished = rfctx(ctx)
			if finished {
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
			finished = rfctx(ctx)
			if finished {
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

func (a ArifmeticProggression) Duration(tm uint64) time.Duration {
	return a.initial + (a.delta * time.Duration(tm))
}

type ConstantProgression time.Duration

func (p ConstantProgression) Duration(uint64) time.Duration {
	return time.Duration(p)
}

type FibonacciProgression time.Duration

func (s FibonacciProgression) Duration(attempt uint64) time.Duration {
	return time.Duration(s) * time.Duration(fibonacciIterative(attempt+1))
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
