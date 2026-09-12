// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package collection

import (
	"container/list"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/onexstack/onexmesh/pkg/core/timex"
)

// drainWorkers is the parallelism used when draining all tasks on Stop.
const drainWorkers = 8

var (
	// ErrClosed indicates the TimingWheel was already stopped.
	ErrClosed = errors.New("collection: timing wheel is closed")
	// ErrArgument indicates an invalid timer argument.
	ErrArgument = errors.New("collection: incorrect timer argument")
)

// Execute runs a fired timer task.
type Execute func(key, value any)

// TimingWheel is a timing wheel that schedules tasks by their delay. A single
// goroutine drives it: ticks advance the wheel, and a handful of channels
// serialize Set/Move/Remove/Drain/Stop operations.
type TimingWheel struct {
	interval      time.Duration
	ticker        timex.Ticker
	slots         []*list.List
	timers        *SafeMap[any, *positionEntry]
	tickedPos     int
	numSlots      int
	execute       Execute
	setChannel    chan timingEntry
	moveChannel   chan baseEntry
	removeChannel chan any
	drainChannel  chan func(key, value any)
	stopChannel   chan struct{}
}

type timingEntry struct {
	baseEntry
	value   any
	circle  int
	diff    int
	removed bool
}

type baseEntry struct {
	delay time.Duration
	key   any
}

type positionEntry struct {
	pos  int
	item *timingEntry
}

type timingTask struct {
	key   any
	value any
}

// NewTimingWheel returns a TimingWheel driven by a real ticker.
func NewTimingWheel(interval time.Duration, numSlots int, execute Execute) (*TimingWheel, error) {
	return NewTimingWheelWithTicker(interval, numSlots, execute, timex.NewTicker(interval))
}

// NewTimingWheelWithTicker returns a TimingWheel driven by the given ticker.
func NewTimingWheelWithTicker(interval time.Duration, numSlots int, execute Execute,
	ticker timex.Ticker) (*TimingWheel, error) {
	if interval <= 0 || numSlots <= 0 || execute == nil {
		return nil, fmt.Errorf("collection: interval=%v slots=%d execute=%p", interval, numSlots, execute)
	}
	tw := &TimingWheel{
		interval:      interval,
		ticker:        ticker,
		slots:         make([]*list.List, numSlots),
		timers:        NewSafeMap[any, *positionEntry](),
		tickedPos:     numSlots - 1,
		execute:       execute,
		numSlots:      numSlots,
		setChannel:    make(chan timingEntry),
		moveChannel:   make(chan baseEntry),
		removeChannel: make(chan any),
		drainChannel:  make(chan func(key, value any)),
		stopChannel:   make(chan struct{}),
	}
	tw.initSlots()
	go tw.run()
	return tw, nil
}

// SetTimer schedules value to fire after delay, keyed by key.
func (tw *TimingWheel) SetTimer(key, value any, delay time.Duration) error {
	if delay <= 0 || key == nil {
		return ErrArgument
	}
	select {
	case tw.setChannel <- timingEntry{
		baseEntry: baseEntry{delay: delay, key: key},
		value:     value,
	}:
		return nil
	case <-tw.stopChannel:
		return ErrClosed
	}
}

// MoveTimer reschedules the task with the given key to a new delay.
func (tw *TimingWheel) MoveTimer(key any, delay time.Duration) error {
	if delay <= 0 || key == nil {
		return ErrArgument
	}
	select {
	case tw.moveChannel <- baseEntry{delay: delay, key: key}:
		return nil
	case <-tw.stopChannel:
		return ErrClosed
	}
}

// RemoveTimer cancels the task with the given key.
func (tw *TimingWheel) RemoveTimer(key any) error {
	if key == nil {
		return ErrArgument
	}
	select {
	case tw.removeChannel <- key:
		return nil
	case <-tw.stopChannel:
		return ErrClosed
	}
}

// Drain fires all pending tasks through fn and returns once finished.
func (tw *TimingWheel) Drain(fn func(key, value any)) error {
	select {
	case tw.drainChannel <- fn:
		return nil
	case <-tw.stopChannel:
		return ErrClosed
	}
}

// Stop stops the wheel and its ticker.
func (tw *TimingWheel) Stop() {
	close(tw.stopChannel)
}

func (tw *TimingWheel) initSlots() {
	for i := 0; i < tw.numSlots; i++ {
		tw.slots[i] = list.New()
	}
}

func (tw *TimingWheel) run() {
	for {
		select {
		case <-tw.ticker.Chan():
			tw.onTick()
		case task := <-tw.setChannel:
			tw.setTask(&task)
		case key := <-tw.removeChannel:
			tw.removeTask(key)
		case task := <-tw.moveChannel:
			tw.moveTask(task)
		case fn := <-tw.drainChannel:
			tw.drainAll(fn)
		case <-tw.stopChannel:
			tw.ticker.Stop()
			return
		}
	}
}

func (tw *TimingWheel) getPositionAndCircle(d time.Duration) (pos, circle int) {
	steps := int(d / tw.interval)
	pos = (tw.tickedPos + steps) % tw.numSlots
	circle = (steps - 1) / tw.numSlots
	return
}

func (tw *TimingWheel) onTick() {
	tw.tickedPos = (tw.tickedPos + 1) % tw.numSlots
	tw.scanAndRunTasks(tw.slots[tw.tickedPos])
}

func (tw *TimingWheel) setTask(task *timingEntry) {
	if task.delay < tw.interval {
		task.delay = tw.interval
	}
	if val, ok := tw.timers.Get(task.key); ok {
		entry := val
		entry.item.value = task.value
		tw.moveTask(task.baseEntry)
	} else {
		pos, circle := tw.getPositionAndCircle(task.delay)
		task.circle = circle
		tw.slots[pos].PushBack(task)
		tw.setTimerPosition(pos, task)
	}
}

func (tw *TimingWheel) moveTask(task baseEntry) {
	val, ok := tw.timers.Get(task.key)
	if !ok {
		return
	}
	timer := val
	if task.delay < tw.interval {
		safeGo(func() { tw.execute(timer.item.key, timer.item.value) })
		return
	}
	pos, circle := tw.getPositionAndCircle(task.delay)
	if pos >= timer.pos {
		timer.item.circle = circle
		timer.item.diff = pos - timer.pos
	} else if circle > 0 {
		circle--
		timer.item.circle = circle
		timer.item.diff = tw.numSlots + pos - timer.pos
	} else {
		timer.item.removed = true
		newItem := &timingEntry{baseEntry: task, value: timer.item.value}
		tw.slots[pos].PushBack(newItem)
		tw.setTimerPosition(pos, newItem)
	}
}

func (tw *TimingWheel) removeTask(key any) {
	val, ok := tw.timers.Get(key)
	if !ok {
		return
	}
	val.item.removed = true
	tw.timers.Del(key)
}

func (tw *TimingWheel) setTimerPosition(pos int, task *timingEntry) {
	if val, ok := tw.timers.Get(task.key); ok {
		val.item = task
		val.pos = pos
	} else {
		tw.timers.Set(task.key, &positionEntry{pos: pos, item: task})
	}
}

func (tw *TimingWheel) scanAndRunTasks(l *list.List) {
	var tasks []timingTask
	for e := l.Front(); e != nil; {
		task := e.Value.(*timingEntry)
		if task.removed {
			next := e.Next()
			l.Remove(e)
			e = next
			continue
		}
		if task.circle > 0 {
			task.circle--
			e = e.Next()
			continue
		}
		if task.diff > 0 {
			next := e.Next()
			l.Remove(e)
			pos := (tw.tickedPos + task.diff) % tw.numSlots
			tw.slots[pos].PushBack(task)
			tw.setTimerPosition(pos, task)
			task.diff = 0
			e = next
			continue
		}
		tasks = append(tasks, timingTask{key: task.key, value: task.value})
		next := e.Next()
		l.Remove(e)
		tw.timers.Del(task.key)
		e = next
	}
	tw.runTasks(tasks)
}

func (tw *TimingWheel) runTasks(tasks []timingTask) {
	if len(tasks) == 0 {
		return
	}
	for _, task := range tasks {
		safeGo(func() { tw.execute(task.key, task.value) })
	}
}

func (tw *TimingWheel) drainAll(fn func(key, value any)) {
	var tasks []timingTask
	for _, slot := range tw.slots {
		for e := slot.Front(); e != nil; {
			task := e.Value.(*timingEntry)
			next := e.Next()
			slot.Remove(e)
			if val, ok := tw.timers.Get(task.key); ok && val.item == task {
				tw.timers.Del(task.key)
			}
			e = next
			if !task.removed {
				tasks = append(tasks, timingTask{key: task.key, value: task.value})
			}
		}
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, drainWorkers)
	for _, task := range tasks {
		wg.Add(1)
		sem <- struct{}{}
		go func(key, value any) {
			defer func() {
				<-sem
				wg.Done()
			}()
			safeGo(func() { fn(key, value) })
		}(task.key, task.value)
	}
	wg.Wait()
}

// safeGo runs fn in a fresh goroutine, recovering and logging any panic so a
// single misbehaving task cannot kill the wheel.
func safeGo(fn func()) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("timing wheel task panicked", "panic", r)
		}
	}()
	fn()
}
