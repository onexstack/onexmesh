// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package executors

import (
	"sync"
	"testing"
	"time"
)

func TestBulkExecutorFlushesOnThreshold(t *testing.T) {
	var mu sync.Mutex
	var got []any
	done := make(chan struct{})

	be := NewBulkExecutor(func(tasks []any) {
		mu.Lock()
		got = append(got, tasks...)
		mu.Unlock()
		close(done)
	}, WithBulkTasks(3), WithBulkInterval(time.Hour))

	be.Add(1)
	be.Add(2)
	be.Add(3) // reaches threshold, flushes

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("bulk executor did not flush on threshold")
	}

	mu.Lock()
	n := len(got)
	mu.Unlock()
	if n != 3 {
		t.Fatalf("executed %d tasks, want 3", n)
	}
}

func TestPeriodicalExecutorFlush(t *testing.T) {
	var mu sync.Mutex
	var got []any

	container := &sliceContainer{execute: func(tasks []any) {
		mu.Lock()
		got = append(got, tasks...)
		mu.Unlock()
	}}
	pe := NewPeriodicalExecutor(time.Hour, container)

	pe.Add(1)
	pe.Add(2)
	pe.Flush()

	// Execute runs asynchronously; poll until it observes the flushed tasks.
	deadline := time.Now().Add(time.Second)
	for {
		mu.Lock()
		n := len(got)
		mu.Unlock()
		if n == 2 || time.Now().After(deadline) {
			break
		}
		time.Sleep(time.Millisecond)
	}

	mu.Lock()
	n := len(got)
	mu.Unlock()
	if n != 2 {
		t.Fatalf("executed %d tasks, want 2", n)
	}
}

func TestDelayExecutorCoalesces(t *testing.T) {
	var mu sync.Mutex
	calls := 0
	de := NewDelayExecutor(func() {
		mu.Lock()
		calls++
		mu.Unlock()
	}, 50*time.Millisecond)

	for i := 0; i < 5; i++ {
		de.Trigger()
	}
	time.Sleep(150 * time.Millisecond)

	mu.Lock()
	n := calls
	mu.Unlock()
	if n != 1 {
		t.Fatalf("fn ran %d times, want 1 (coalesced)", n)
	}
}

type sliceContainer struct {
	tasks   []any
	execute func([]any)
}

func (c *sliceContainer) AddTask(task any) bool {
	c.tasks = append(c.tasks, task)
	return false
}

func (c *sliceContainer) Execute(tasks any) {
	c.execute(tasks.([]any))
}

func (c *sliceContainer) RemoveAll() any {
	tasks := c.tasks
	c.tasks = nil
	return tasks
}
