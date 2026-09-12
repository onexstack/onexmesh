// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package mr

import (
	"context"
	"errors"
	"testing"
)

func TestMapReduce(t *testing.T) {
	// Sum of squares of 1..100 via a MapReduce.
	result, err := MapReduce(
		func(source chan<- int) {
			for i := 1; i <= 100; i++ {
				source <- i
			}
		},
		func(item int, writer Writer[int], _ func(error)) {
			writer.Write(item * item)
		},
		func(pipe <-chan int, writer Writer[int], _ func(error)) {
			sum := 0
			for v := range pipe {
				sum += v
			}
			writer.Write(sum)
		},
		WithWorkers(8),
	)
	if err != nil {
		t.Fatal(err)
	}
	if result != 338350 { // 100*101*201/6
		t.Fatalf("result = %d, want 338350", result)
	}
}

func TestMapReduceCancel(t *testing.T) {
	want := errors.New("cancel")
	_, err := MapReduce(
		func(source chan<- int) {
			for i := 0; i < 100; i++ {
				source <- i
			}
		},
		func(item int, _ Writer[int], cancel func(error)) {
			if item == 5 {
				cancel(want)
				return
			}
		},
		func(_ <-chan int, _ Writer[int], _ func(error)) {},
		WithWorkers(4),
	)
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
}

func TestMapReduceContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := MapReduce(
		func(source chan<- int) {
			for i := 0; i < 10; i++ {
				source <- i
			}
		},
		func(item int, _ Writer[int], _ func(error)) {},
		func(_ <-chan int, _ Writer[int], _ func(error)) {},
		WithContext(ctx),
	)
	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context cancellation", err)
	}
}

func TestFinish(t *testing.T) {
	want := errors.New("boom")
	err := Finish(
		func() error { return nil },
		func() error { return want },
	)
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
}
