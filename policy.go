package retry

import (
	"context"
	"errors"
	"slices"
	"time"
)

var ErrRetryCountExceeded = errors.New("retry count exceeded")

type Result struct {
	code    code
	backoff time.Duration
	err     error
}

func (r Result) Finished() bool {
	return r.code == codeFinished
}

func (r Result) Backoff() time.Duration {
	return r.backoff
}

func (r Result) Err() error {
	return r.err
}

func Continue() Result {
	return Result{}
}

func Recover(recoverErr error) Result {
	return Result{
		err:     recoverErr,
		code:    codeContinue,
		backoff: 0,
	}
}

func RecoverAfter(recoverErr error, backoff time.Duration) Result {
	return Result{
		err:     recoverErr,
		code:    codeContinue,
		backoff: backoff,
	}
}

func Abort(err error) Result {
	return Result{
		err:     err,
		code:    codeFinished,
		backoff: 0,
	}
}

func RetryAfter(backoff time.Duration) Result {
	return Result{
		err:     nil,
		code:    codeContinue,
		backoff: backoff,
	}
}

func Finish() Result {
	return Result{
		err:     nil,
		code:    codeFinished,
		backoff: 0,
	}
}

type code uint8

const (
	codeContinue code = iota
	codeFinished
)

type (
	Backoff    func(uint64) time.Duration
	Func       func(context.Context) Result
	Middleware func(Func) Func
)

func WithBackoff(backoff Backoff) Middleware {
	return func(rf Func) Func {
		attempt := uint64(0)

		return func(ctx context.Context) Result {
			res := rf(ctx)

			attempt, res = backoffMiddleware(attempt, backoff, res)

			return res
		}
	}
}

func WithMaxRetryCount(maxRetryCount uint64) Middleware {
	return func(rf Func) Func {
		attempt := uint64(0)

		return func(ctx context.Context) Result {
			res := rf(ctx)

			attempt, res = retryCountExceededMiddleware(attempt, maxRetryCount, res)

			return res
		}
	}
}

func backoffMiddleware(attempt uint64, backoff Backoff, res Result) (uint64, Result) {
	if res.code != codeContinue {
		return attempt, res
	}

	if res.backoff != 0 {
		attempt++

		return attempt, res
	}

	res.backoff = backoff(attempt)
	attempt++

	return attempt, res
}

func retryCountExceededMiddleware(attempt, maxRetryCount uint64, res Result) (uint64, Result) {
	if res.code != codeContinue {
		return attempt, res
	}

	if attempt < maxRetryCount {
		attempt++

		return attempt, res
	}

	switch res.err {
	case nil:
		res = Abort(ErrRetryCountExceeded)
	default:
		res = Abort(errors.Join(ErrRetryCountExceeded, res.err))
	}

	return attempt, res
}

func Retry(ctx context.Context, rf Func, middlewares ...Middleware) error {
	for _, mdw := range slices.Backward(middlewares) {
		rf = mdw(rf)
	}

	res := rf(ctx)
	if res.code != codeContinue {
		return res.Err()
	}

	for {
		sleepTime := res.backoff

		if sleepTime <= 0 {
			res = rf(ctx)
			if res.code != codeContinue {
				return res.Err()
			}

			continue
		}

		timer := time.NewTimer(sleepTime)

		select {
		case <-ctx.Done():
			timer.Stop()

			abortErr := context.Cause(ctx)
			if res.err != nil {
				abortErr = errors.Join(abortErr, res.err)
			}

			return abortErr
		case <-timer.C:
			res = rf(ctx)
			if res.code != codeContinue {
				return res.Err()
			}
		}
	}
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
