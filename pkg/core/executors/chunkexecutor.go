// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package executors

import "time"

const defaultChunkSize = 1024 * 1024 // 1MiB

// ChunkOption customizes a ChunkExecutor.
type ChunkOption func(*chunkOptions)

// ChunkExecutor executes tasks when either the accumulated chunk size is
// reached or the flush interval elapses.
type ChunkExecutor struct {
	executor  *PeriodicalExecutor
	container *chunkContainer
}

type chunkOptions struct {
	chunkSize     int
	flushInterval time.Duration
}

// NewChunkExecutor returns a ChunkExecutor that executes chunks with execute.
func NewChunkExecutor(execute Execute, opts ...ChunkOption) *ChunkExecutor {
	options := newChunkOptions()
	for _, opt := range opts {
		opt(&options)
	}

	container := &chunkContainer{execute: execute, maxChunkSize: options.chunkSize}
	return &ChunkExecutor{
		executor:  NewPeriodicalExecutor(options.flushInterval, container),
		container: container,
	}
}

// Add adds a task with the given byte size to the chunk.
func (ce *ChunkExecutor) Add(task any, size int) error {
	ce.executor.Add(chunk{val: task, size: size})
	return nil
}

// Flush forces a flush.
func (ce *ChunkExecutor) Flush() {
	ce.executor.Flush()
}

// Wait flushes and waits for execution to finish.
func (ce *ChunkExecutor) Wait() {
	ce.executor.Wait()
}

// WithChunkBytes sets the chunk size that triggers a flush.
func WithChunkBytes(size int) ChunkOption {
	return func(o *chunkOptions) { o.chunkSize = size }
}

// WithFlushInterval sets the flush interval.
func WithFlushInterval(duration time.Duration) ChunkOption {
	return func(o *chunkOptions) { o.flushInterval = duration }
}

func newChunkOptions() chunkOptions {
	return chunkOptions{chunkSize: defaultChunkSize, flushInterval: defaultFlushInterval}
}

type chunkContainer struct {
	tasks        []any
	execute      Execute
	size         int
	maxChunkSize int
}

func (bc *chunkContainer) AddTask(task any) bool {
	ck := task.(chunk)
	bc.tasks = append(bc.tasks, ck.val)
	bc.size += ck.size
	return bc.size >= bc.maxChunkSize
}

func (bc *chunkContainer) Execute(tasks any) {
	bc.execute(tasks.([]any))
}

func (bc *chunkContainer) RemoveAll() any {
	tasks := bc.tasks
	bc.tasks = nil
	bc.size = 0
	return tasks
}

type chunk struct {
	val  any
	size int
}
