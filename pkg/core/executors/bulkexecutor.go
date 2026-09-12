// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package executors

import "time"

const defaultBulkTasks = 1000

// BulkOption customizes a BulkExecutor.
type BulkOption func(*bulkOptions)

// BulkExecutor executes tasks when either the batch size is reached or the
// flush interval elapses.
type BulkExecutor struct {
	executor  *PeriodicalExecutor
	container *bulkContainer
}

type bulkOptions struct {
	cachedTasks   int
	flushInterval time.Duration
}

// NewBulkExecutor returns a BulkExecutor that executes batches with execute.
func NewBulkExecutor(execute Execute, opts ...BulkOption) *BulkExecutor {
	options := newBulkOptions()
	for _, opt := range opts {
		opt(&options)
	}

	container := &bulkContainer{execute: execute, maxTasks: options.cachedTasks}
	return &BulkExecutor{
		executor:  NewPeriodicalExecutor(options.flushInterval, container),
		container: container,
	}
}

// Add adds a task to the batch.
func (be *BulkExecutor) Add(task any) error {
	be.executor.Add(task)
	return nil
}

// Flush forces a flush.
func (be *BulkExecutor) Flush() {
	be.executor.Flush()
}

// Wait flushes and waits for execution to finish.
func (be *BulkExecutor) Wait() {
	be.executor.Wait()
}

// WithBulkTasks sets the batch size that triggers a flush.
func WithBulkTasks(tasks int) BulkOption {
	return func(o *bulkOptions) { o.cachedTasks = tasks }
}

// WithBulkInterval sets the flush interval.
func WithBulkInterval(duration time.Duration) BulkOption {
	return func(o *bulkOptions) { o.flushInterval = duration }
}

func newBulkOptions() bulkOptions {
	return bulkOptions{cachedTasks: defaultBulkTasks, flushInterval: defaultFlushInterval}
}

type bulkContainer struct {
	tasks    []any
	execute  Execute
	maxTasks int
}

func (bc *bulkContainer) AddTask(task any) bool {
	bc.tasks = append(bc.tasks, task)
	return len(bc.tasks) >= bc.maxTasks
}

func (bc *bulkContainer) Execute(tasks any) {
	bc.execute(tasks.([]any))
}

func (bc *bulkContainer) RemoveAll() any {
	tasks := bc.tasks
	bc.tasks = nil
	return tasks
}
