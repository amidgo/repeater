package retry_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/amidgo/retry"
)

func Test_Backoff(t *testing.T) {
	tests := []struct {
		BackoffName string
		Backoff     retry.Backoff
		Attempt     uint64
		Expected    time.Duration
	}{
		{
			BackoffName: "retry.Pain(time.Second)",
			Backoff:     retry.Plain(time.Second),
			Attempt:     132,
			Expected:    time.Second,
		},
		{
			BackoffName: "retry.Plain(time.Second)",
			Backoff:     retry.Plain(time.Second),
			Attempt:     1,
			Expected:    time.Second,
		},
		{
			BackoffName: "retry.Plain(time.Second * 10)",
			Backoff:     retry.Plain(time.Second * 10),
			Attempt:     1,
			Expected:    time.Second * 10,
		},
		{
			BackoffName: "retry.Arifmetic(time.Second, time.Second*2)",
			Backoff:     retry.Arifmetic(time.Second, time.Second*2),
			Attempt:     1,
			Expected:    time.Second * 3,
		},
		{
			BackoffName: "retry.Arifmetic(time.Second, time.Second*2)",
			Backoff:     retry.Arifmetic(time.Second, time.Second*2),
			Attempt:     0,
			Expected:    time.Second,
		},
		{
			BackoffName: "retry.Arifmetic(time.Second, time.Second*2)",
			Backoff:     retry.Arifmetic(time.Second, time.Second*2),
			Attempt:     0,
			Expected:    time.Second,
		},
		{
			BackoffName: "retry.Arifmetic(time.Second, time.Second*2)",
			Backoff:     retry.Arifmetic(time.Second, time.Second*2),
			Attempt:     3,
			Expected:    time.Second * 7,
		},
		{
			BackoffName: "retry.Fibonacci(time.Second)",
			Backoff:     retry.Fibonacci(time.Second),
			Attempt:     0,
			Expected:    time.Second,
		},
		{
			BackoffName: "retry.Fibonacci(time.Second)",
			Backoff:     retry.Fibonacci(time.Second),
			Attempt:     1,
			Expected:    time.Second,
		},
		{
			BackoffName: "retry.Fibonacci(time.Second)",
			Backoff:     retry.Fibonacci(time.Second),
			Attempt:     2,
			Expected:    time.Second * 2,
		},
		{
			BackoffName: "retry.Fibonacci(time.Second)",
			Backoff:     retry.Fibonacci(time.Second),
			Attempt:     3,
			Expected:    time.Second * 3,
		},
		{
			BackoffName: "retry.Fibonacci(time.Second)",
			Backoff:     retry.Fibonacci(time.Second),
			Attempt:     4,
			Expected:    time.Second * 5,
		},
		{
			BackoffName: "retry.Fibonacci(time.Second)",
			Backoff:     retry.Fibonacci(time.Second),
			Attempt:     5,
			Expected:    time.Second * 8,
		},
		{
			BackoffName: "retry.Fibonacci(time.Second)",
			Backoff:     retry.Fibonacci(time.Second),
			Attempt:     6,
			Expected:    time.Second * 13,
		},
		{
			BackoffName: "retry.Fibonacci(time.Second)",
			Backoff:     retry.Fibonacci(time.Second),
			Attempt:     7,
			Expected:    time.Second * 21,
		},
		{
			BackoffName: "retry.Fibonacci(time.Second)",
			Backoff:     retry.Fibonacci(time.Second),
			Attempt:     8,
			Expected:    time.Second * 34,
		},
		{
			BackoffName: "retry.Fibonacci(time.Second)",
			Backoff:     retry.Fibonacci(time.Second),
			Attempt:     9,
			Expected:    time.Second * 55,
		},
	}

	for _, tst := range tests {
		t.Run(fmt.Sprintf("%s backoff, %d attempt", tst.BackoffName, tst.Attempt),
			func(t *testing.T) {
				backoff := tst.Backoff(tst.Attempt)

				if backoff != tst.Expected {
					t.Fatalf("compare backoff duration\n\nexpected:\n%s\n\nactual:\n%s", tst.Expected, backoff)
				}
			},
		)
	}
}
