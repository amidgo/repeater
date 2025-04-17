package retry

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var ErrRetryCountExceeded = errors.New("retry count exceeded")

type Result struct {
	code Code
	err  error
}

func (r Result) Code() Code {
	return r.code
}

func (r Result) Err() error {
	return r.err
}

func Continue() Result {
	return Result{
		code: CodeContinue,
	}
}

func Abort(err error) Result {
	return Result{
		code: CodeAborted,
		err:  err,
	}
}

func Finish() Result {
	return Result{
		code: CodeFinished,
	}
}

func retryCountExceeded() Result {
	return Result{
		code: CodeRetryCountExceeded,
		err:  ErrRetryCountExceeded,
	}
}

type Code uint8

const (
	CodeContinue Code = iota
	CodeAborted
	CodeRetryCountExceeded
	CodeFinished
)

func (c Code) String() string {
	switch c {
	case CodeContinue:
		return "Continue"
	case CodeAborted:
		return "Aborted"
	case CodeRetryCountExceeded:
		return "CodeRetryCountExceeded"
	case CodeFinished:
		return "Finished"
	default:
		return fmt.Sprintf("UNKNOWN<%d>", c)
	}
}

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

	Func        func() Result
	FuncContext func(ctx context.Context) Result
)

func Retry(progression DurationProgression, retryCount uint64, f Func) Result {
	policy := New(progression, retryCount)

	return policy.Retry(f)
}

func RetryContext(ctx context.Context, progression DurationProgression, retryCount uint64, f FuncContext) Result {
	policy := New(progression, retryCount)

	return policy.RetryContext(ctx, f)
}

type Policy struct {
	progression DurationProgression
	retryCount  uint64
}

func New(progression DurationProgression, retryCount uint64) Policy {
	return Policy{progression: progression, retryCount: retryCount}
}

func (r Policy) Retry(rf Func) (result Result) {
	result = rf()
	if result.Code() != CodeContinue {
		return result
	}

	for attempt := range r.retryCount {
		sleepTime := r.progression.Duration(attempt)
		if sleepTime <= 0 {
			result = rf()
			if result.Code() != CodeContinue {
				return result
			}

			continue
		}

		<-time.After(sleepTime)

		result = rf()
		if result.Code() != CodeContinue {
			return result
		}
	}

	return retryCountExceeded()
}

func (r Policy) RetryContext(ctx context.Context, rfctx FuncContext) (result Result) {
	result = rfctx(ctx)
	if result.Code() != CodeContinue {
		return result
	}

	for attempt := range r.retryCount {
		sleepTime := r.progression.Duration(attempt)
		if sleepTime <= 0 {
			result = rfctx(ctx)
			if result.Code() != CodeContinue {
				return result
			}

			continue
		}

		timer := time.NewTimer(sleepTime)

		select {
		case <-ctx.Done():
			timer.Stop()

			return Abort(context.Cause(ctx))
		case <-timer.C:
			result = rfctx(ctx)
			if result.Code() != CodeContinue {
				return result
			}
		}
	}

	return retryCountExceeded()
}

type arifmeticProgression struct {
	initial time.Duration
	delta   time.Duration
}

func ArifmeticProgression(initial, delta time.Duration) arifmeticProgression {
	return arifmeticProgression{initial: initial, delta: delta}
}

func (a arifmeticProgression) Duration(tm uint64) time.Duration {
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
