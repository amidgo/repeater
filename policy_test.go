package retry_test

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/amidgo/retry"
	"github.com/amidgo/tester"
)

type RetryTest struct {
	Name                   string
	Progression            retry.DurationProgression
	RetryCount             uint64
	RepeatOperations       RetryOperations
	ExpectedCode           retry.Code
	ExpectedErr            error
	ExpectedRepeatDuration time.Duration
}

func (r *RetryTest) Test(t *testing.T) {
	t.Parallel()

	t.Run("method", r.runMethodTest)
	t.Run("global func", r.runGlobalFuncTest)
}

func (r *RetryTest) runMethodTest(t *testing.T) {
	t.Parallel()

	repeatOperations := r.RepeatOperations.Copy()

	now := time.Now()

	rp := retry.New(r.Progression, r.RetryCount)

	result := rp.Retry(repeatOperations.Execute())
	if r.ExpectedCode != result.Code() {
		t.Fatalf("wrong result code, expected %s, actual %s", r.ExpectedCode, result.Code())
	}

	if r.ExpectedErr != result.Err() {
		t.Fatalf("wrong result err, expected %+v, actual %+v", r.ExpectedErr, result.Err())
	}

	finishTime := time.Now()

	diff := finishTime.Sub(now) - r.ExpectedRepeatDuration

	if diff.Abs() > time.Millisecond*10 {
		t.Fatalf("too big difference between actual and expected repeat time: %s", diff)
	}
}

func (r *RetryTest) runGlobalFuncTest(t *testing.T) {
	t.Parallel()

	repeatOperations := r.RepeatOperations.Copy()

	now := time.Now()

	result := retry.Retry(r.Progression, r.RetryCount, repeatOperations.Execute())
	if r.ExpectedCode != result.Code() {
		t.Fatalf("wrong result code, expected %s, actual %s", r.ExpectedCode, result.Code())
	}

	if r.ExpectedErr != result.Err() {
		t.Fatalf("wrong result err, expected %+v, actual %+v", r.ExpectedErr, result.Err())
	}

	finishTime := time.Now()

	diff := finishTime.Sub(now) - r.ExpectedRepeatDuration

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
			RepeatOperations: NewRetryOperations(
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
			ExpectedCode:           retry.CodeRetryCountExceeded,
			ExpectedErr:            retry.ErrRetryCountExceeded,
		},
		&RetryTest{
			Name:        "zero delay repeat",
			Progression: retry.ConstantProgression(0),
			RetryCount:  2,
			RepeatOperations: NewRetryOperations(
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
			ExpectedRepeatDuration: time.Second * 2,
			ExpectedCode:           retry.CodeRetryCountExceeded,
			ExpectedErr:            retry.ErrRetryCountExceeded,
		},
		&RetryTest{
			Name:        "success repeat after first call",
			Progression: retry.ConstantProgression(time.Second),
			RetryCount:  2,
			RepeatOperations: NewRetryOperations(
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
			ExpectedRepeatDuration: time.Second * 2,
			ExpectedCode:           retry.CodeFinished,
			ExpectedErr:            nil,
		},
		&RetryTest{
			Name:        "success repeat after first call, retry after",
			Progression: retry.ConstantProgression(time.Second * 2),
			RetryCount:  2,
			RepeatOperations: NewRetryOperations(
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
			ExpectedCode:           retry.CodeFinished,
			ExpectedErr:            nil,
		},
		&RetryTest{
			Name:        "zero repeat count",
			Progression: retry.ConstantProgression(time.Second),
			RetryCount:  0,
			RepeatOperations: NewRetryOperations(
				RetryOperation{
					Duration: time.Millisecond * 500,
					Result:   retry.Continue(),
				},
			),
			ExpectedRepeatDuration: time.Millisecond * 500,
			ExpectedCode:           retry.CodeRetryCountExceeded,
			ExpectedErr:            retry.ErrRetryCountExceeded,
		},
		&RetryTest{
			Name:        "aborted with error",
			Progression: retry.ConstantProgression(time.Second),
			RetryCount:  1,
			RepeatOperations: NewRetryOperations(
				RetryOperation{
					Duration: time.Millisecond * 500,
					Result:   retry.Abort(io.ErrUnexpectedEOF),
				},
			),
			ExpectedRepeatDuration: time.Millisecond * 500,
			ExpectedCode:           retry.CodeAborted,
			ExpectedErr:            io.ErrUnexpectedEOF,
		},
	)
}

type RepeatContextTest struct {
	CaseName               string
	Progression            retry.DurationProgression
	RetryCount             uint64
	ContextTimeout         time.Duration
	ContextCause           error
	RetryOperations        RetryOperations
	ExpectedCode           retry.Code
	ExpectedErr            error
	ExpectedRepeatDuration time.Duration
}

func (r *RepeatContextTest) Name() string {
	return r.CaseName
}

func (r *RepeatContextTest) Test(t *testing.T) {
	t.Parallel()

	t.Run("method", r.runMethodTest)
	t.Run("global func", r.runGlobalFuncTest)
}

func (r *RepeatContextTest) runMethodTest(t *testing.T) {
	t.Parallel()

	repeatOperations := r.RetryOperations.Copy()

	now := time.Now()

	ctx, cancel := context.WithDeadlineCause(context.Background(), now.Add(r.ContextTimeout), r.ContextCause)
	defer cancel()

	rp := retry.New(r.Progression, r.RetryCount)

	result := rp.RetryContext(ctx, repeatOperations.ExecuteContext())
	if r.ExpectedCode != result.Code() {
		t.Fatalf("wrong result code, expected %s, actual %s", r.ExpectedCode, result.Code())
	}

	if r.ExpectedErr != result.Err() {
		t.Fatalf("wrong result err, expected %+v, actual %+v", r.ExpectedErr, result.Err())
	}

	finishTime := time.Now()

	diff := finishTime.Sub(now) - r.ExpectedRepeatDuration

	if diff.Abs() > time.Millisecond*10 {
		t.Fatalf("too big difference between actual and expected repeat time: %s", diff)
	}
}

func (r *RepeatContextTest) runGlobalFuncTest(t *testing.T) {
	t.Parallel()

	repeatOperations := r.RetryOperations.Copy()

	now := time.Now()

	ctx, cancel := context.WithDeadlineCause(context.Background(), now.Add(r.ContextTimeout), r.ContextCause)
	defer cancel()

	result := retry.RetryContext(ctx, r.Progression, r.RetryCount, repeatOperations.ExecuteContext())
	if r.ExpectedCode != result.Code() {
		t.Fatalf("wrong result code, expected %s, actual %s", r.ExpectedCode, result.Code())
	}

	if r.ExpectedErr != result.Err() {
		t.Fatalf("wrong result err, expected %+v, actual %+v", r.ExpectedErr, result.Err())
	}

	finishTime := time.Now()

	diff := finishTime.Sub(now) - r.ExpectedRepeatDuration

	if diff.Abs() > time.Millisecond*10 {
		t.Fatalf("too big difference between actual and expected repeat time: %s", diff)
	}
}

func Test_RepeatContext(t *testing.T) {
	t.Parallel()

	tester.RunNamedTesters(t,
		&RepeatContextTest{
			CaseName:       "basic repeat",
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
			ExpectedCode:           retry.CodeRetryCountExceeded,
			ExpectedErr:            retry.ErrRetryCountExceeded,
		},
		&RepeatContextTest{
			CaseName:       "basic repeat, context canceled after 1.75 seconds during execute",
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
			ExpectedCode:           retry.CodeAborted,
			ExpectedErr:            io.ErrUnexpectedEOF,
		},
		&RepeatContextTest{
			CaseName:       "basic repeat, context canceled after 1.75 seconds during execute, retry after",
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
			ExpectedCode:           retry.CodeAborted,
			ExpectedErr:            io.ErrUnexpectedEOF,
		},
		&RepeatContextTest{
			CaseName:       "basic repeat, context canceled after 2.5 seconds during pause",
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
			ExpectedCode:           retry.CodeAborted,
			ExpectedErr:            context.DeadlineExceeded,
		},
		&RepeatContextTest{
			CaseName:       "success repeat after first call",
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
			ExpectedCode:           retry.CodeFinished,
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
		<-time.After(r.Duration)

		return r.Result
	}
}

func (r RetryOperation) ExecuteContext() func(context.Context) retry.Result {
	return func(ctx context.Context) retry.Result {
		select {
		case <-time.After(r.Duration):
			return r.Result
		case <-ctx.Done():
			return retry.Abort(context.Cause(ctx))
		}
	}
}
