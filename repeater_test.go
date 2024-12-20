package repeater_test

import (
	"context"
	"testing"
	"time"

	"github.com/amidgo/repeater"
	"github.com/amidgo/tester"
)

type RepeatTest struct {
	CaseName               string
	Progression            repeater.DurationProgression
	RepeatCount            uint64
	RepeatOperations       RepeatOperations
	ExpectedFinished       bool
	ExpectedRepeatDuration time.Duration
}

func (r *RepeatTest) Name() string {
	return r.CaseName
}

func (r *RepeatTest) Test(t *testing.T) {
	t.Parallel()

	t.Run("method", r.runMethodTest)
	t.Run("global func", r.runGlobalFuncTest)
}

func (r *RepeatTest) runMethodTest(t *testing.T) {
	t.Parallel()

	repeatOperations := r.RepeatOperations.Copy()

	now := time.Now()

	rp := repeater.New(r.Progression)

	finished := rp.Repeat(repeatOperations.Execute(), r.RepeatCount)
	if r.ExpectedFinished != finished {
		t.Fatalf("wrong success, expect %t, actual %t", r.ExpectedFinished, finished)
	}

	finishTime := time.Now()

	diff := finishTime.Sub(now) - r.ExpectedRepeatDuration

	if diff.Abs() > time.Millisecond*10 {
		t.Fatalf("too big difference between actual and expected repeat time: %s", diff)
	}
}

func (r *RepeatTest) runGlobalFuncTest(t *testing.T) {
	t.Parallel()

	repeatOperations := r.RepeatOperations.Copy()

	now := time.Now()

	finished := repeater.Repeat(r.Progression, repeatOperations.Execute(), r.RepeatCount)
	if r.ExpectedFinished != finished {
		t.Fatalf("wrong success, expect %t, actual %t", r.ExpectedFinished, finished)
	}

	finishTime := time.Now()

	diff := finishTime.Sub(now) - r.ExpectedRepeatDuration

	if diff.Abs() > time.Millisecond*10 {
		t.Fatalf("too big difference between actual and expected repeat time: %s", diff)
	}
}

func Test_Repeat(t *testing.T) {
	t.Parallel()

	tester.RunNamedTesters(t,
		&RepeatTest{
			CaseName:    "basic repeat",
			Progression: repeater.ConstantProgression(time.Second),
			RepeatCount: 2,
			RepeatOperations: NewRepeatOperaions(
				RepeatOperation{
					Duration: time.Millisecond * 500,
					OK:       false,
				},
				// 1 second pause
				RepeatOperation{
					Duration: time.Millisecond * 500,
					OK:       false,
				},
				// 1 second pause
				RepeatOperation{
					Duration: time.Millisecond * 1000,
					OK:       false,
				},
			),
			ExpectedRepeatDuration: time.Second * 4,
			ExpectedFinished:       false,
		},
		&RepeatTest{
			CaseName:    "zero delay repeat",
			Progression: repeater.ConstantProgression(0),
			RepeatCount: 2,
			RepeatOperations: NewRepeatOperaions(
				RepeatOperation{
					Duration: time.Millisecond * 500,
					OK:       false,
				},
				// 1 second pause
				RepeatOperation{
					Duration: time.Millisecond * 500,
					OK:       false,
				},
				// 1 second pause
				RepeatOperation{
					Duration: time.Millisecond * 1000,
					OK:       false,
				},
			),
			ExpectedRepeatDuration: time.Second * 2,
			ExpectedFinished:       false,
		},
		&RepeatTest{
			CaseName:    "success repeat after first call",
			Progression: repeater.ConstantProgression(time.Second),
			RepeatCount: 2,
			RepeatOperations: NewRepeatOperaions(
				RepeatOperation{
					Duration: time.Millisecond * 500,
					OK:       false,
				},
				// 1 second pause
				RepeatOperation{
					Duration: time.Millisecond * 500,
					OK:       true,
				},
				// 1 second pause
				RepeatOperation{
					Duration: time.Millisecond * 1000,
					OK:       false,
				},
			),
			ExpectedRepeatDuration: time.Second * 2,
			ExpectedFinished:       true,
		},
		&RepeatTest{
			CaseName:    "zero repeat count",
			Progression: repeater.ConstantProgression(time.Second),
			RepeatCount: 0,
			RepeatOperations: NewRepeatOperaions(
				RepeatOperation{
					Duration: time.Millisecond * 500,
					OK:       false,
				},
			),
			ExpectedRepeatDuration: time.Millisecond * 500,
			ExpectedFinished:       false,
		},
	)
}

type RepeatContextTest struct {
	CaseName               string
	Progression            repeater.DurationProgression
	RepeatCount            uint64
	ContextTimeout         time.Duration
	RepeatOperations       RepeatOperations
	ExpectedFinished       bool
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

	repeatOperations := r.RepeatOperations.Copy()

	now := time.Now()

	ctx, cancel := context.WithDeadline(context.Background(), now.Add(r.ContextTimeout))
	defer cancel()

	rp := repeater.New(r.Progression)

	finished := rp.RepeatContext(ctx, repeatOperations.ExecuteContext(), r.RepeatCount)
	if r.ExpectedFinished != finished {
		t.Fatalf("wrong success, expect %t, actual %t", r.ExpectedFinished, finished)
	}

	finishTime := time.Now()

	diff := finishTime.Sub(now) - r.ExpectedRepeatDuration

	if diff.Abs() > time.Millisecond*10 {
		t.Fatalf("too big difference between actual and expected repeat time: %s", diff)
	}
}

func (r *RepeatContextTest) runGlobalFuncTest(t *testing.T) {
	t.Parallel()

	repeatOperations := r.RepeatOperations.Copy()

	now := time.Now()

	ctx, cancel := context.WithDeadline(context.Background(), now.Add(r.ContextTimeout))
	defer cancel()

	finished := repeater.RepeatContext(ctx, r.Progression, repeatOperations.ExecuteContext(), r.RepeatCount)
	if r.ExpectedFinished != finished {
		t.Fatalf("wrong success, expect %t, actual %t", r.ExpectedFinished, finished)
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
			Progression:    repeater.ConstantProgression(time.Second),
			RepeatCount:    2,
			ContextTimeout: time.Second * 5,
			RepeatOperations: NewRepeatOperaions(
				RepeatOperation{
					Duration: time.Millisecond * 500,
					OK:       false,
				},
				// 1 second pause
				RepeatOperation{
					Duration: time.Millisecond * 500,
					OK:       false,
				},
				// 1 second pause
				RepeatOperation{
					Duration: time.Millisecond * 1000,
					OK:       false,
				},
			),
			ExpectedRepeatDuration: time.Second * 4,
			ExpectedFinished:       false,
		},
		&RepeatContextTest{
			CaseName:       "basic repeat, context canceled after 1.75 seconds during execute",
			Progression:    repeater.ConstantProgression(time.Second),
			RepeatCount:    2,
			ContextTimeout: time.Millisecond * 1750,
			RepeatOperations: NewRepeatOperaions(
				RepeatOperation{
					Duration: time.Millisecond * 500,
					OK:       false,
				},
				// 1 second pause
				RepeatOperation{
					Duration: time.Millisecond * 500,
					OK:       true,
				},
				// 1 second pause
				RepeatOperation{
					Duration: time.Millisecond * 1000,
					OK:       false,
				},
			),
			ExpectedRepeatDuration: time.Millisecond * 1750,
			ExpectedFinished:       false,
		},
		&RepeatContextTest{
			CaseName:       "basic repeat, context canceled after 2.5 seconds during pause",
			Progression:    repeater.ConstantProgression(time.Second),
			RepeatCount:    2,
			ContextTimeout: time.Millisecond * 2500,
			RepeatOperations: NewRepeatOperaions(
				RepeatOperation{
					Duration: time.Millisecond * 500,
					OK:       false,
				},
				// 1 second pause
				RepeatOperation{
					Duration: time.Millisecond * 500,
					OK:       false,
				},
				// 1 second pause
				RepeatOperation{
					Duration: time.Millisecond * 1000,
					OK:       false,
				},
			),
			ExpectedRepeatDuration: time.Millisecond * 2500,
			ExpectedFinished:       false,
		},
		&RepeatContextTest{
			CaseName:       "success repeat after first call",
			Progression:    repeater.ConstantProgression(time.Second),
			RepeatCount:    2,
			ContextTimeout: time.Second * 3,
			RepeatOperations: NewRepeatOperaions(
				RepeatOperation{
					Duration: time.Millisecond * 500,
					OK:       false,
				},
				// 1 second pause
				RepeatOperation{
					Duration: time.Millisecond * 500,
					OK:       true,
				},
				// 1 second pause
				RepeatOperation{
					Duration: time.Millisecond * 1000,
					OK:       false,
				},
			),
			ExpectedRepeatDuration: time.Second * 2,
			ExpectedFinished:       true,
		},
	)
}

type RepeatOperations struct {
	ops []RepeatOperation
}

func (r RepeatOperations) Copy() RepeatOperations {
	ops := make([]RepeatOperation, len(r.ops))

	copy(ops, r.ops)

	return RepeatOperations{ops: ops}
}

func NewRepeatOperaions(ops ...RepeatOperation) RepeatOperations {
	return RepeatOperations{ops: ops}
}

func (r *RepeatOperations) Execute() func() bool {
	return func() bool {
		op := r.pop()

		return op.Execute()()
	}
}

func (r *RepeatOperations) ExecuteContext() func(context.Context) bool {
	return func(ctx context.Context) bool {
		op := r.pop()

		return op.ExecuteContext()(ctx)
	}
}

func (r *RepeatOperations) pop() RepeatOperation {
	op := r.ops[0]
	r.ops = r.ops[1:]

	return op
}

type RepeatOperation struct {
	Duration time.Duration
	OK       bool
}

func (r RepeatOperation) Execute() func() bool {
	return func() bool {
		<-time.After(r.Duration)

		return r.OK
	}
}

func (r RepeatOperation) ExecuteContext() func(context.Context) bool {
	return func(ctx context.Context) bool {
		select {
		case <-time.After(r.Duration):
			return r.OK
		case <-ctx.Done():
			return false
		}
	}
}
