package retry

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var ErrRetryCountExceeded = errors.New("retry count exceeded")

type Result struct {
	code       code
	retryAfter time.Duration
	err        error
}

func (r Result) Eq(result Result) (bool, string) {
	if r.code != result.code {
		return false, fmt.Sprintf("compare 'code', original: %s, other: %s", r.code, result.code)
	}

	if r.retryAfter != result.retryAfter {
		return false, fmt.Sprintf("compare 'retryAfter', original: %s, other: %s", r.retryAfter, result.retryAfter)
	}

	if r.err != result.err {
		return false, fmt.Sprintf("compare 'err', original: %s, other: %s", r.err, result.err)
	}

	return true, ""
}

func (r Result) Err() error {
	return r.err
}

func Continue() Result {
	return Result{}
}

func Recover(recoverErr error) Result {
	return Result{
		err: recoverErr,
	}
}

func Abort(err error) Result {
	return Result{
		code: codeFinished,
		err:  err,
	}
}

func RetryAfter(sleepDuration time.Duration) Result {
	return Result{
		retryAfter: sleepDuration,
	}
}

func Finish() Result {
	return Result{
		code: codeFinished,
	}
}

type code uint8

const (
	codeContinue code = iota
	codeFinished
)

func (c code) String() string {
	switch c {
	case codeContinue:
		return "continue"
	case codeFinished:
		return "finished"
	default:
		panic("unexpected code value: " + fmt.Sprint(int(c)))
	}
}

type (
	Backoff     func(uint64) time.Duration
	Func        func() Result
	FuncContext func(ctx context.Context) Result
)

func Retry(backoff Backoff, retryCount uint64, f Func) error {
	policy := New(backoff, retryCount)

	return policy.Retry(f)
}

func RetryContext(ctx context.Context, backoff Backoff, retryCount uint64, f FuncContext) error {
	policy := New(backoff, retryCount)

	return policy.RetryContext(ctx, f)
}

type Policy struct {
	backoff    Backoff
	retryCount uint64
}

func New(backoff Backoff, retryCount uint64) Policy {
	return Policy{backoff: backoff, retryCount: retryCount}
}

func (r Policy) Retry(rf Func) error {
	result := rf()
	if result.code != codeContinue {
		return result.Err()
	}

	for attempt := range r.retryCount {
		sleepTime := r.backoff(attempt)
		if result.retryAfter != 0 {
			sleepTime = result.retryAfter
		}

		if sleepTime <= 0 {
			result = rf()
			if result.code != codeContinue {
				return result.Err()
			}

			continue
		}

		<-time.After(sleepTime)

		result = rf()
		if result.code != codeContinue {
			return result.Err()
		}
	}

	err := ErrRetryCountExceeded
	if result.err != nil {
		err = errors.Join(err, result.err)
	}

	return err
}

func (r Policy) RetryContext(ctx context.Context, rfctx FuncContext) error {
	result := rfctx(ctx)
	if result.code != codeContinue {
		return result.Err()
	}

	for attempt := range r.retryCount {
		sleepTime := r.backoff(attempt)

		if result.retryAfter != 0 {
			sleepTime = result.retryAfter
		}

		if sleepTime <= 0 {
			result = rfctx(ctx)
			if result.code != codeContinue {
				return result.Err()
			}

			continue
		}

		timer := time.NewTimer(sleepTime)

		select {
		case <-ctx.Done():
			timer.Stop()

			abortErr := context.Cause(ctx)
			if result.err != nil {
				abortErr = errors.Join(abortErr, result.err)
			}

			return abortErr
		case <-timer.C:
			result = rfctx(ctx)
			if result.code != codeContinue {
				return result.Err()
			}
		}
	}

	err := ErrRetryCountExceeded
	if result.err != nil {
		err = errors.Join(err, result.err)
	}

	return err
}

func Plain(backoff time.Duration) Backoff {
	return func(attempt uint64) time.Duration {
		return backoff
	}
}

func Arifmetic(initial, delta time.Duration) Backoff {
	return func(attempt uint64) time.Duration {
		return initial + (delta * time.Duration(attempt))
	}
}

func Fibonacci(base time.Duration) Backoff {
	return func(attempt uint64) time.Duration {
		return base * time.Duration(fibonacciIterative(attempt+1))
	}
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
