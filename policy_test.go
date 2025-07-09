package retry_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/amidgo/retry"
)

type RetryTest struct {
	Name                  string
	Backoff               retry.Backoff
	RetryCount            uint64
	RetryOperations       RetryOperations
	ExpectedErr           error
	ExpectedRetryDuration time.Duration
}

func (r *RetryTest) Test(t *testing.T) {
	t.Parallel()

	t.Run("method", r.runMethodTest)
	t.Run("global func", r.runGlobalFuncTest)
}

func (r *RetryTest) runMethodTest(t *testing.T) {
	t.Parallel()

	repeatOperations := r.RetryOperations.Copy()

	now := time.Now()

	rp := retry.New(r.Backoff, r.RetryCount)

	result := rp.Retry(repeatOperations.Execute())
	assertResultError(t, r.ExpectedErr, result.Err())

	finishTime := time.Now()

	diff := finishTime.Sub(now) - r.ExpectedRetryDuration

	if diff.Abs() > time.Millisecond*10 {
		t.Fatalf("too big difference between actual and expected repeat time: %s", diff)
	}
}

func (r *RetryTest) runGlobalFuncTest(t *testing.T) {
	t.Parallel()

	repeatOperations := r.RetryOperations.Copy()

	now := time.Now()

	result := retry.Retry(r.Backoff, r.RetryCount, repeatOperations.Execute())
	assertResultError(t, r.ExpectedErr, result.Err())

	finishTime := time.Now()

	diff := finishTime.Sub(now) - r.ExpectedRetryDuration

	if diff.Abs() > time.Millisecond*10 {
		t.Fatalf("too big difference between actual and expected repeat time: %s", diff)
	}
}

func Test_Retry(t *testing.T) {
	t.Parallel()

	tests := []*RetryTest{
		{
			Name:       "basic repeat",
			Backoff:    retry.Plain(time.Second),
			RetryCount: 2,
			RetryOperations: NewRetryOperations(
				RetryOperation{
					Duration: time.Millisecond * 500,
					Result:   retry.Continue(),
				},
				// 1 second pause
				RetryOperation{
					Duration: time.Millisecond * 500,
					Result:   retry.Continue(),
				},
				// 1 second pause
				RetryOperation{
					Duration: time.Millisecond * 1000,
					Result:   retry.Continue(),
				},
			),
			ExpectedRetryDuration: time.Second * 4,
			ExpectedErr:           retry.ErrRetryCountExceeded,
		},
		{
			Name:       "zero delay repeat",
			Backoff:    retry.Plain(0),
			RetryCount: 2,
			RetryOperations: NewRetryOperations(
				RetryOperation{
					Duration: time.Millisecond * 500,
					Result:   retry.Continue(),
				},
				// 1 second pause
				RetryOperation{
					Duration: time.Millisecond * 500,
					Result:   retry.Continue(),
				},
				// 1 second pause
				RetryOperation{
					Duration: time.Millisecond * 1000,
					Result:   retry.Continue(),
				},
			),
			ExpectedRetryDuration: time.Second * 2,
			ExpectedErr:           retry.ErrRetryCountExceeded,
		},
		{
			Name:       "success repeat after first call",
			Backoff:    retry.Plain(time.Second),
			RetryCount: 2,
			RetryOperations: NewRetryOperations(
				RetryOperation{
					Duration: time.Millisecond * 500,
					Result:   retry.Continue(),
				},
				// 1 second pause
				RetryOperation{
					Duration: time.Millisecond * 500,
					Result:   retry.Finish(),
				},
				// 1 second pause
				RetryOperation{
					Duration: time.Millisecond * 1000,
					Result:   retry.Continue(),
				},
			),
			ExpectedRetryDuration: time.Second * 2,
			ExpectedErr:           nil,
		},
		{
			Name:       "success repeat after first call, retry after",
			Backoff:    retry.Plain(time.Second * 2),
			RetryCount: 2,
			RetryOperations: NewRetryOperations(
				RetryOperation{
					Duration: time.Millisecond * 500,
					Result:   retry.RetryAfter(time.Second),
				},
				RetryOperation{
					Duration: 0,
					// negative duration for retry immediately
					Result: retry.RetryAfter(-time.Second),
				},
				// 1 second pause
				RetryOperation{
					Duration: time.Millisecond * 500,
					Result:   retry.Finish(),
				},
				// 1 second pause
				RetryOperation{
					Duration: time.Millisecond * 1000,
					Result:   retry.Continue(),
				},
			),
			ExpectedRetryDuration: time.Second * 2,
			ExpectedErr:           nil,
		},
		{
			Name:       "zero repeat count",
			Backoff:    retry.Plain(time.Second),
			RetryCount: 0,
			RetryOperations: NewRetryOperations(
				RetryOperation{
					Duration: time.Millisecond * 500,
					Result:   retry.Continue(),
				},
			),
			ExpectedRetryDuration: time.Millisecond * 500,
			ExpectedErr:           retry.ErrRetryCountExceeded,
		},
		{
			Name:       "aborted with error",
			Backoff:    retry.Plain(time.Second),
			RetryCount: 1,
			RetryOperations: NewRetryOperations(
				RetryOperation{
					Duration: time.Millisecond * 500,
					Result:   retry.Abort(io.ErrUnexpectedEOF),
				},
			),
			ExpectedRetryDuration: time.Millisecond * 500,
			ExpectedErr:           io.ErrUnexpectedEOF,
		},
		{
			Name:       "abort after recover",
			Backoff:    retry.Plain(time.Second),
			RetryCount: 2,
			RetryOperations: NewRetryOperations(
				RetryOperation{
					Duration: time.Millisecond * 500,
					Result:   retry.Recover(io.ErrUnexpectedEOF),
				},
				// 1 second pause
				RetryOperation{
					Duration: time.Millisecond * 500,
					Result:   retry.Abort(io.ErrShortWrite),
				},
			),
			ExpectedRetryDuration: time.Second * 2,
			ExpectedErr:           io.ErrShortWrite,
		},
		{
			Name:       "success after recover",
			Backoff:    retry.Plain(time.Second),
			RetryCount: 2,
			RetryOperations: NewRetryOperations(
				RetryOperation{
					Duration: time.Millisecond * 500,
					Result:   retry.Recover(io.ErrUnexpectedEOF),
				},
				RetryOperation{
					Duration: 0,
					Result:   retry.Finish(),
				},
			),
			ExpectedRetryDuration: time.Millisecond * 1500,
			ExpectedErr:           nil,
		},
		{
			Name:       "retry count excedeed with Recover",
			Backoff:    retry.Plain(time.Second),
			RetryCount: 1,
			RetryOperations: NewRetryOperations(
				RetryOperation{
					Duration: 0,
					Result:   retry.Recover(io.ErrUnexpectedEOF),
				},
				RetryOperation{
					Duration: 0,
					Result:   retry.Recover(io.ErrUnexpectedEOF),
				},
			),
			ExpectedRetryDuration: time.Second,
			ExpectedErr:           errors.Join(retry.ErrRetryCountExceeded, io.ErrUnexpectedEOF),
		},
	}

	for _, tst := range tests {
		t.Run(tst.Name, tst.Test)
	}
}

type RetryContextTest struct {
	Name                  string
	Backoff               retry.Backoff
	RetryCount            uint64
	ContextTimeout        time.Duration
	ContextCause          error
	RetryOperations       RetryOperations
	ExpectedErr           error
	ExpectedRetryDuration time.Duration
}

func (r *RetryContextTest) Test(t *testing.T) {
	t.Parallel()

	t.Run("retry.New().RetryContext()", r.runContextMethodTest)
	t.Run("retry.RetryContext()", r.runContextFuncTest)
}

func (r *RetryContextTest) runContextMethodTest(t *testing.T) {
	t.Parallel()

	repeatOperations := r.RetryOperations.Copy()

	now := time.Now()

	ctx, cancel := context.WithDeadlineCause(context.Background(), now.Add(r.ContextTimeout), r.ContextCause)
	defer cancel()

	rp := retry.New(r.Backoff, r.RetryCount)

	result := rp.RetryContext(ctx, repeatOperations.ExecuteContext())
	assertResultError(t, r.ExpectedErr, result.Err())

	finishTime := time.Now()

	diff := finishTime.Sub(now) - r.ExpectedRetryDuration

	if diff.Abs() > time.Millisecond*10 {
		t.Fatalf("too big difference between actual and expected repeat time: %s", diff)
	}
}

func (r *RetryContextTest) runContextFuncTest(t *testing.T) {
	t.Parallel()

	repeatOperations := r.RetryOperations.Copy()

	now := time.Now()

	ctx, cancel := context.WithDeadlineCause(context.Background(), now.Add(r.ContextTimeout), r.ContextCause)
	defer cancel()

	result := retry.RetryContext(ctx, r.Backoff, r.RetryCount, repeatOperations.ExecuteContext())
	assertResultError(t, r.ExpectedErr, result.Err())

	finishTime := time.Now()

	diff := finishTime.Sub(now) - r.ExpectedRetryDuration

	if diff.Abs() > time.Millisecond*10 {
		t.Fatalf("too big difference between actual and expected repeat time: %s", diff)
	}
}

func assertResultError(t *testing.T, expectedErr, resultErr error) {
	if expectedErr == resultErr {
		return
	}

	expectedErrs := extractErrors(expectedErr)
	resultErrs := extractErrors(resultErr)

	if len(expectedErrs) != len(resultErrs) {
		t.Fatalf("wrong result err\n\nexpected:\n%+v\n\nactual:\n%+v", expectedErr, resultErr)

		return
	}

	for i := range expectedErrs {
		if expectedErrs[i] != resultErrs[i] {
			t.Fatalf("wrong result err, check %d element of errs\n\nexpected:\n%+v\n\nactual:\n%+v", i+1, expectedErrs[i], resultErrs[i])

			return
		}
	}
}

func extractErrors(expectedErr error) []error {
	switch expectedErr := expectedErr.(type) {
	case interface{ Unwrap() []error }:
		return expectedErr.Unwrap()
	case interface{ Unwrap() error }:
		return []error{expectedErr.Unwrap()}
	default:
		return []error{expectedErr}
	}
}

func Test_RetryContext(t *testing.T) {
	t.Parallel()

	tests := []*RetryContextTest{
		{
			Name:           "basic repeat",
			Backoff:        retry.Plain(time.Second),
			RetryCount:     2,
			ContextTimeout: time.Second * 5,
			RetryOperations: NewRetryOperations(
				RetryOperation{
					Duration: time.Millisecond * 500,
					Result:   retry.Continue(),
				},
				// 1 second pause
				RetryOperation{
					Duration: time.Millisecond * 500,
					Result:   retry.Continue(),
				},
				// 1 second pause
				RetryOperation{
					Duration: time.Millisecond * 1000,
					Result:   retry.Continue(),
				},
			),
			ExpectedRetryDuration: time.Second * 4,
			ExpectedErr:           retry.ErrRetryCountExceeded,
		},
		{
			Name:           "basic repeat, context canceled after 1.75 seconds during execute",
			Backoff:        retry.Plain(time.Second),
			RetryCount:     2,
			ContextTimeout: time.Millisecond * 1750,
			ContextCause:   io.ErrUnexpectedEOF,
			RetryOperations: NewRetryOperations(
				RetryOperation{
					Duration: time.Millisecond * 500,
					Result:   retry.Continue(),
				},
				// 1 second pause
				RetryOperation{
					Duration: time.Millisecond * 500,
					Result:   retry.Continue(),
				},
				// 1 second pause
				RetryOperation{
					Duration: time.Millisecond * 1000,
					Result:   retry.Continue(),
				},
			),
			ExpectedRetryDuration: time.Millisecond * 1750,
			ExpectedErr:           io.ErrUnexpectedEOF,
		},
		{
			Name:           "basic repeat, context canceled after 1.75 seconds during execute, retry after",
			Backoff:        retry.Plain(0),
			RetryCount:     2,
			ContextTimeout: time.Millisecond * 1750,
			ContextCause:   io.ErrUnexpectedEOF,
			RetryOperations: NewRetryOperations(
				RetryOperation{
					Duration: time.Millisecond * 500,
					Result:   retry.RetryAfter(time.Second),
				},
				// 1 second pause
				RetryOperation{
					Duration: time.Millisecond * 500,
					Result:   retry.Continue(),
				},
				// 1 second pause
				RetryOperation{
					Duration: time.Millisecond * 1000,
					Result:   retry.Continue(),
				},
			),
			ExpectedRetryDuration: time.Millisecond * 1750,
			ExpectedErr:           io.ErrUnexpectedEOF,
		},
		{
			Name:           "basic repeat, context canceled after 2.5 seconds during pause",
			Backoff:        retry.Plain(time.Second),
			RetryCount:     2,
			ContextTimeout: time.Millisecond * 2500,
			RetryOperations: NewRetryOperations(
				RetryOperation{
					Duration: time.Millisecond * 500,
					Result:   retry.Continue(),
				},
				// 1 second pause
				RetryOperation{
					Duration: time.Millisecond * 500,
					Result:   retry.Continue(),
				},
				// 1 second pause
				RetryOperation{
					Duration: time.Millisecond * 1000,
					Result:   retry.Continue(),
				},
			),
			ExpectedRetryDuration: time.Millisecond * 2500,
			ExpectedErr:           context.DeadlineExceeded,
		},
		{
			Name:           "recover with context canceled after 2.5 seconds during pause",
			Backoff:        retry.Plain(time.Second),
			RetryCount:     2,
			ContextTimeout: time.Millisecond * 2500,
			RetryOperations: NewRetryOperations(
				RetryOperation{
					Duration: time.Millisecond * 500,
					Result:   retry.Continue(),
				},
				// 1 second pause
				RetryOperation{
					Duration: time.Millisecond * 500,
					Result:   retry.Recover(io.ErrUnexpectedEOF),
				},
				// 1 second pause, not called
				RetryOperation{
					Duration: time.Millisecond * 1000,
					Result:   retry.Finish(),
				},
			),
			ExpectedRetryDuration: time.Millisecond * 2500,
			ExpectedErr:           errors.Join(context.DeadlineExceeded, io.ErrUnexpectedEOF),
		},
		{
			Name:           "continue after recover, context canceled after 2.5 seconds during pause",
			Backoff:        retry.Plain(time.Second),
			RetryCount:     2,
			ContextTimeout: time.Millisecond * 2500,
			RetryOperations: NewRetryOperations(
				RetryOperation{
					Duration: time.Millisecond * 500,
					Result:   retry.Recover(io.ErrUnexpectedEOF),
				},
				// 1 second pause
				RetryOperation{
					Duration: time.Millisecond * 500,
					Result:   retry.Continue(),
				},
				// 1 second pause, not called
				RetryOperation{
					Duration: time.Millisecond * 1000,
					Result:   retry.Continue(),
				},
			),
			ExpectedRetryDuration: time.Millisecond * 2500,
			ExpectedErr:           context.DeadlineExceeded,
		},
		{
			Name:           "abort after recover",
			Backoff:        retry.Plain(time.Second),
			RetryCount:     2,
			ContextTimeout: time.Millisecond * 2500,
			RetryOperations: NewRetryOperations(
				RetryOperation{
					Duration: time.Millisecond * 500,
					Result:   retry.Recover(io.ErrUnexpectedEOF),
				},
				// 1 second pause
				RetryOperation{
					Duration: time.Millisecond * 500,
					Result:   retry.Abort(io.ErrShortWrite),
				},
			),
			ExpectedRetryDuration: time.Second * 2,
			ExpectedErr:           io.ErrShortWrite,
		},
		{
			Name:           "success after recover",
			Backoff:        retry.Plain(time.Second),
			RetryCount:     2,
			ContextTimeout: time.Millisecond * 2500,
			RetryOperations: NewRetryOperations(
				RetryOperation{
					Duration: time.Millisecond * 500,
					Result:   retry.Recover(io.ErrUnexpectedEOF),
				},
				RetryOperation{
					Duration: 0,
					Result:   retry.Finish(),
				},
			),
			ExpectedRetryDuration: time.Millisecond * 1500,
			ExpectedErr:           nil,
		},
		{
			Name:           "success after recover + retry.RetryAfter(0)",
			Backoff:        retry.Plain(time.Second),
			RetryCount:     2,
			ContextTimeout: time.Millisecond * 2500,
			RetryOperations: NewRetryOperations(
				RetryOperation{
					Duration: time.Millisecond * 500,
					Result:   retry.Recover(io.ErrUnexpectedEOF),
				},
				RetryOperation{
					Duration: time.Millisecond * 500,
					// RetryAfter(-1) for immediatly retry
					Result: retry.RetryAfter(-1),
				},
				RetryOperation{
					Duration: 0,
					Result:   retry.Finish(),
				},
			),
			ExpectedRetryDuration: time.Second * 2,
			ExpectedErr:           nil,
		},
		{
			Name:           "success repeat after first call",
			Backoff:        retry.Plain(time.Second * 5),
			RetryCount:     2,
			ContextTimeout: time.Second * 3,
			RetryOperations: NewRetryOperations(
				RetryOperation{
					Duration: time.Millisecond * 500,
					Result:   retry.RetryAfter(time.Second),
				},
				// 1 second pause
				RetryOperation{
					Duration: time.Millisecond * 500,
					Result:   retry.Finish(),
				},
				// 1 second pause
				RetryOperation{
					Duration: time.Millisecond * 1000,
					Result:   retry.Continue(),
				},
			),
			ExpectedRetryDuration: time.Second * 2,
			ExpectedErr:           nil,
		},
		{
			Name:           "retry count excedeed with Recover",
			Backoff:        retry.Plain(time.Second),
			RetryCount:     1,
			ContextTimeout: time.Second * 10,
			RetryOperations: NewRetryOperations(
				RetryOperation{
					Duration: 0,
					Result:   retry.Recover(io.ErrUnexpectedEOF),
				},
				RetryOperation{
					Duration: 0,
					Result:   retry.Recover(io.ErrUnexpectedEOF),
				},
			),
			ExpectedRetryDuration: time.Second,
			ExpectedErr:           errors.Join(retry.ErrRetryCountExceeded, io.ErrUnexpectedEOF),
		},
		{
			Name:           "immediately finish",
			Backoff:        retry.Plain(time.Second),
			RetryCount:     0,
			ContextTimeout: time.Second * 10,
			RetryOperations: NewRetryOperations(
				RetryOperation{
					Duration: time.Second,
					Result:   retry.Finish(),
				},
			),
			ExpectedRetryDuration: time.Second,
			ExpectedErr:           nil,
		},
		{
			Name:           "several retry after in a row",
			Backoff:        retry.Plain(time.Second),
			RetryCount:     3,
			ContextTimeout: time.Second * 10,
			RetryOperations: NewRetryOperations(
				RetryOperation{
					Duration: 0,
					Result:   retry.RetryAfter(-1),
				},
				RetryOperation{
					Duration: 0,
					Result:   retry.RetryAfter(-1),
				},
				RetryOperation{
					Duration: 0,
					Result:   retry.RetryAfter(-1),
				},
				RetryOperation{
					Duration: 0,
					Result:   retry.Finish(),
				},
			),
			ExpectedRetryDuration: 0,
			ExpectedErr:           nil,
		},
	}

	for _, tst := range tests {
		t.Run(tst.Name, tst.Test)
	}
}

type RetryOperations struct {
	ops []RetryOperation
}

func (r RetryOperations) Copy() RetryOperations {
	ops := make([]RetryOperation, len(r.ops))

	copy(ops, r.ops)

	return RetryOperations{ops: ops}
}

func NewRetryOperations(ops ...RetryOperation) RetryOperations {
	return RetryOperations{ops: ops}
}

func (r *RetryOperations) Execute() func() retry.Result {
	return func() retry.Result {
		op := r.pop()

		return op.Execute()()
	}
}

func (r *RetryOperations) ExecuteContext() func(context.Context) retry.Result {
	return func(ctx context.Context) retry.Result {
		op := r.pop()

		return op.ExecuteContext()(ctx)
	}
}

func (r *RetryOperations) pop() RetryOperation {
	op := r.ops[0]
	r.ops = r.ops[1:]

	return op
}

type RetryOperation struct {
	Duration time.Duration
	Result   retry.Result
}

func (r RetryOperation) Execute() func() retry.Result {
	return func() retry.Result {
		if r.Duration == 0 {
			return r.Result
		}

		<-time.After(r.Duration)

		return r.Result
	}
}

func (r RetryOperation) ExecuteContext() func(context.Context) retry.Result {
	return func(ctx context.Context) retry.Result {
		if r.Duration == 0 {
			return r.Result
		}

		select {
		case <-time.After(r.Duration):
			return r.Result
		case <-ctx.Done():
			return retry.Abort(context.Cause(ctx))
		}
	}
}

type resultEqTest struct {
	Name            string
	Original, Other retry.Result
	ExpectedEqual   bool
	ExpectedMessage string
}

func (r *resultEqTest) Test(t *testing.T) {
	equal, message := r.Original.Eq(r.Other)

	if equal != r.ExpectedEqual {
		t.Fatalf("compare equal, expected: %t, actual: %t", r.ExpectedEqual, equal)
	}

	if message != r.ExpectedMessage {
		t.Fatalf("compare message, message not equal\n\nexpected:\n%s\n\nactual:\n%s", r.ExpectedMessage, message)
	}
}

func Test_Result_Eq(t *testing.T) {
	tests := []*resultEqTest{
		{
			Name:            "code not equal",
			Original:        retry.Continue(),
			Other:           retry.Finish(),
			ExpectedEqual:   false,
			ExpectedMessage: "compare 'code', original: continue, other: finished",
		},
		{
			Name:            "retryAfter not equal",
			Original:        retry.RetryAfter(time.Second),
			Other:           retry.RetryAfter(time.Second * 2),
			ExpectedEqual:   false,
			ExpectedMessage: "compare 'retryAfter', original: 1s, other: 2s",
		},
		{
			Name:            "err not equal",
			Original:        retry.Recover(io.ErrUnexpectedEOF),
			Other:           retry.Recover(io.ErrNoProgress),
			ExpectedEqual:   false,
			ExpectedMessage: "compare 'err', original: unexpected EOF, other: multiple Read calls return no data or error",
		},
		{
			Name:            "wrapped err not equal",
			Original:        retry.Recover(io.ErrUnexpectedEOF),
			Other:           retry.Recover(fmt.Errorf("wrapped: %w", io.ErrUnexpectedEOF)),
			ExpectedEqual:   false,
			ExpectedMessage: "compare 'err', original: unexpected EOF, other: wrapped: unexpected EOF",
		},
		{
			Name:            "equal, retryAfter",
			Original:        retry.RetryAfter(time.Second),
			Other:           retry.RetryAfter(time.Second),
			ExpectedEqual:   true,
			ExpectedMessage: "",
		},
		{
			Name:            "equal, err",
			Original:        retry.Recover(io.ErrUnexpectedEOF),
			Other:           retry.Recover(io.ErrUnexpectedEOF),
			ExpectedEqual:   true,
			ExpectedMessage: "",
		},
	}

	for _, tst := range tests {
		t.Run(tst.Name, tst.Test)
	}
}
