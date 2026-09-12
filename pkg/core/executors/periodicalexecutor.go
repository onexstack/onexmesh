// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package executors

import (
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/onexstack/onexmesh/pkg/core/syncx"
	"github.com/onexstack/onexmesh/pkg/core/timex"
)

// idleRound is the number of idle intervals after which a PeriodicalExecutor's
// background flusher exits.
const idleRound = 10

// TaskContainer accumulates tasks for a PeriodicalExecutor.
type TaskContainer interface {
	// AddTask adds task, returning true if the container needs flushing now.
	AddTask(task any) bool
	// Execute handles the collected tasks.
	Execute(tasks any)
	// RemoveAll removes and returns all collected tasks.
	RemoveAll() any
}

// PeriodicalExecutor coalesces tasks and executes them periodically, or
// immediately when the container signals fullness.
type PeriodicalExecutor struct {
	commander   chan any
	interval    time.Duration
	container   TaskContainer
	waitGroup   sync.WaitGroup
	wgBarrier   syncx.Barrier
	confirmChan chan struct{}
	inflight    int32
	guarded     bool
	newTicker   func(time.Duration) timex.Ticker
	lock        sync.Mutex
}

// NewPeriodicalExecutor returns a PeriodicalExecutor with the given interval and
// container. Call Flush (or Wait) before shutdown to drain pending tasks.
func NewPeriodicalExecutor(interval time.Duration, container TaskContainer) *PeriodicalExecutor {
	executor := &PeriodicalExecutor{
		commander:   make(chan any, 1),
		interval:    interval,
		container:   container,
		confirmChan: make(chan struct{}),
		newTicker: func(d time.Duration) timex.Ticker {
			return timex.NewTicker(d)
		},
	}
	return executor
}

// Add adds a task to be executed.
func (pe *PeriodicalExecutor) Add(task any) {
	if vals, ok := pe.addAndCheck(task); ok {
		pe.commander <- vals
		<-pe.confirmChan
	}
}

// Flush forces pe to execute all pending tasks.
func (pe *PeriodicalExecutor) Flush() bool {
	pe.enterExecution()
	return pe.executeTasks(func() any {
		pe.lock.Lock()
		defer pe.lock.Unlock()
		return pe.container.RemoveAll()
	}())
}

// Sync runs fn while holding the executor's container lock.
func (pe *PeriodicalExecutor) Sync(fn func()) {
	pe.lock.Lock()
	defer pe.lock.Unlock()
	fn()
}

// Wait flushes and waits for all executions to finish.
func (pe *PeriodicalExecutor) Wait() {
	pe.Flush()
	pe.wgBarrier.Guard(func() {
		pe.waitGroup.Wait()
	})
}

func (pe *PeriodicalExecutor) addAndCheck(task any) (any, bool) {
	pe.lock.Lock()
	defer func() {
		if !pe.guarded {
			pe.guarded = true
			// Defer so the flusher starts after the lock is released.
			defer pe.backgroundFlush()
		}
		pe.lock.Unlock()
	}()

	if pe.container.AddTask(task) {
		atomic.AddInt32(&pe.inflight, 1)
		return pe.container.RemoveAll(), true
	}
	return nil, false
}

func (pe *PeriodicalExecutor) backgroundFlush() {
	safeGo(func() {
		// Flush before the goroutine exits so no task is missed.
		defer pe.Flush()

		ticker := pe.newTicker(pe.interval)
		defer ticker.Stop()

		var commanded bool
		last := time.Now()
		for {
			select {
			case vals := <-pe.commander:
				commanded = true
				atomic.AddInt32(&pe.inflight, -1)
				pe.enterExecution()
				pe.confirmChan <- struct{}{}
				pe.executeTasks(vals)
				last = time.Now()
			case <-ticker.Chan():
				if commanded {
					commanded = false
				} else if pe.Flush() {
					last = time.Now()
				} else if pe.shallQuit(last) {
					return
				}
			}
		}
	})
}

func (pe *PeriodicalExecutor) doneExecution() {
	pe.waitGroup.Done()
}

func (pe *PeriodicalExecutor) enterExecution() {
	pe.wgBarrier.Guard(func() {
		pe.waitGroup.Add(1)
	})
}

func (pe *PeriodicalExecutor) executeTasks(tasks any) bool {
	defer pe.doneExecution()

	ok := pe.hasTasks(tasks)
	if ok {
		safeGo(func() {
			pe.container.Execute(tasks)
		})
	}
	return ok
}

func (pe *PeriodicalExecutor) hasTasks(tasks any) bool {
	if tasks == nil {
		return false
	}
	val := reflect.ValueOf(tasks)
	switch val.Kind() {
	case reflect.Array, reflect.Chan, reflect.Map, reflect.Slice:
		return val.Len() > 0
	default:
		// Unknown type; let the caller execute it.
		return true
	}
}

func (pe *PeriodicalExecutor) shallQuit(last time.Time) bool {
	if time.Since(last) <= pe.interval*idleRound {
		return false
	}

	pe.lock.Lock()
	if atomic.LoadInt32(&pe.inflight) == 0 {
		pe.guarded = false
		pe.lock.Unlock()
		return true
	}
	pe.lock.Unlock()
	return false
}
