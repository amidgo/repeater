package retry_test

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/amidgo/retry"
)

type RetryTest struct {
	Name                  string
	Progression           retry.DurationProgression
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

	rp := retry.New(r.Progression, r.RetryCount)

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

	result := retry.Retry(r.Progression, r.RetryCount, repeatOperations.Execute())
	assertResultError(t, r.ExpectedErr, result.Err())

	finishTime := time.Now()

	diff := finishTime.Sub(now) - r.ExpectedRetryDuration

	if diff.Abs() > time.Millisecond*10 {
		t.Fatalf("too big difference between actual and expected repeat time: %s", diff)
	}
}

func runRetryTests(t *testing.T, tests ...*RetryTest) {
	for _, tst := range tests {
		t.Run(tst.Name, tst.Test)
	}
}

func Test_Retry(t *testing.T) {
	t.Parallel()

	runRetryTests(t,
		&RetryTest{
			Name:        "basic repeat",
			Progression: retry.ConstantProgression(time.Second),
			RetryCount:  2,
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
		&RetryTest{
			Name:        "zero delay repeat",
			Progression: retry.ConstantProgression(0),
			RetryCount:  2,
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
		&RetryTest{
			Name:        "success repeat after first call",
			Progression: retry.ConstantProgression(time.Second),
			RetryCount:  2,
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
		&RetryTest{
			Name:        "success repeat after first call, retry after",
			Progression: retry.ConstantProgression(time.Second * 2),
			RetryCount:  2,
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
		&RetryTest{
			Name:        "zero repeat count",
			Progression: retry.ConstantProgression(time.Second),
			RetryCount:  0,
			RetryOperations: NewRetryOperations(
				RetryOperation{
					Duration: time.Millisecond * 500,
					Result:   retry.Continue(),
				},
			),
			ExpectedRetryDuration: time.Millisecond * 500,
			ExpectedErr:           retry.ErrRetryCountExceeded,
		},
		&RetryTest{
			Name:        "aborted with error",
			Progression: retry.ConstantProgression(time.Second),
			RetryCount:  1,
			RetryOperations: NewRetryOperations(
				RetryOperation{
					Duration: time.Millisecond * 500,
					Result:   retry.Abort(io.ErrUnexpectedEOF),
				},
			),
			ExpectedRetryDuration: time.Millisecond * 500,
			ExpectedErr:           io.ErrUnexpectedEOF,
		},
		&RetryTest{
			Name:        "abort after recover",
			Progression: retry.ConstantProgression(time.Second),
			RetryCount:  2,
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
		&RetryTest{
			Name:        "success after recover",
			Progression: retry.ConstantProgression(time.Second),
			RetryCount:  2,
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
	)
}

type RetryContextTest struct {
	Name                   string
	Progression            retry.DurationProgression
	RetryCount             uint64
	ContextTimeout         time.Duration
	ContextCause           error
	RetryOperations        RetryOperations
	ExpectedErr            error
	ExpectedRepeatDuration time.Duration
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

	rp := retry.New(r.Progression, r.RetryCount)

	result := rp.RetryContext(ctx, repeatOperations.ExecuteContext())
	assertResultError(t, r.ExpectedErr, result.Err())

	finishTime := time.Now()

	diff := finishTime.Sub(now) - r.ExpectedRepeatDuration

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

	result := retry.RetryContext(ctx, r.Progression, r.RetryCount, repeatOperations.ExecuteContext())
	assertResultError(t, r.ExpectedErr, result.Err())

	finishTime := time.Now()

	diff := finishTime.Sub(now) - r.ExpectedRepeatDuration

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
		t.Fatalf("wrong result err, expected %+v, actual %+v", expectedErr, resultErr)

		return
	}

	for i := range expectedErrs {
		if expectedErrs[i] != resultErrs[i] {
			t.Fatalf("wrong result err, check %d element of errs, expected %+v, actual %+v", i+1, expectedErrs[i], resultErrs[i])

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

func runRetryContextTests(t *testing.T, tests ...*RetryContextTest) {
	for _, tst := range tests {
		t.Run(tst.Name, tst.Test)
	}
}

func Test_RetryContext(t *testing.T) {
	t.Parallel()

	runRetryContextTests(t,
		&RetryContextTest{
			Name:           "basic repeat",
			Progression:    retry.ConstantProgression(time.Second),
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
			ExpectedRepeatDuration: time.Second * 4,
			ExpectedErr:            retry.ErrRetryCountExceeded,
		},
		&RetryContextTest{
			Name:           "basic repeat, context canceled after 1.75 seconds during execute",
			Progression:    retry.ConstantProgression(time.Second),
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
			ExpectedRepeatDuration: time.Millisecond * 1750,
			ExpectedErr:            io.ErrUnexpectedEOF,
		},
		&RetryContextTest{
			Name:           "basic repeat, context canceled after 1.75 seconds during execute, retry after",
			Progression:    retry.ConstantProgression(0),
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
			ExpectedRepeatDuration: time.Millisecond * 1750,
			ExpectedErr:            io.ErrUnexpectedEOF,
		},
		&RetryContextTest{
			Name:           "basic repeat, context canceled after 2.5 seconds during pause",
			Progression:    retry.ConstantProgression(time.Second),
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
			ExpectedRepeatDuration: time.Millisecond * 2500,
			ExpectedErr:            context.DeadlineExceeded,
		},
		&RetryContextTest{
			Name:           "recover with context canceled after 2.5 seconds during pause",
			Progression:    retry.ConstantProgression(time.Second),
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
			ExpectedRepeatDuration: time.Millisecond * 2500,
			ExpectedErr:            errors.Join(context.DeadlineExceeded, io.ErrUnexpectedEOF),
		},
		&RetryContextTest{
			Name:           "continue after recover, context canceled after 2.5 seconds during pause",
			Progression:    retry.ConstantProgression(time.Second),
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
			ExpectedRepeatDuration: time.Millisecond * 2500,
			ExpectedErr:            context.DeadlineExceeded,
		},
		&RetryContextTest{
			Name:           "abort after recover",
			Progression:    retry.ConstantProgression(time.Second),
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
			ExpectedRepeatDuration: time.Second * 2,
			ExpectedErr:            io.ErrShortWrite,
		},
		&RetryContextTest{
			Name:           "success after recover",
			Progression:    retry.ConstantProgression(time.Second),
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
			ExpectedRepeatDuration: time.Millisecond * 1500,
			ExpectedErr:            nil,
		},
		&RetryContextTest{
			Name:           "success after recover + retry.RetryAfter(0)",
			Progression:    retry.ConstantProgression(time.Second),
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
			ExpectedRepeatDuration: time.Second * 2,
			ExpectedErr:            nil,
		},
		&RetryContextTest{
			Name:           "success repeat after first call",
			Progression:    retry.ConstantProgression(time.Second * 5),
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
			ExpectedRepeatDuration: time.Second * 2,
			ExpectedErr:            nil,
		},
	)
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
