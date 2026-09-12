// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package mathx provides small numeric helpers used by the core components: a
// deviation-based jitter around a value and a probabilistic boolean.
package mathx

import (
	"math/rand"
	"sync"
	"time"
)

// Unstable generates values jittered around a base value by a deviation ratio,
// used to spread cache expiry so many items do not expire at once.
type Unstable struct {
	deviation float64
	r         *rand.Rand
	lock      *sync.Mutex
}

// NewUnstable returns an Unstable with the given deviation, clamped to [0, 1].
func NewUnstable(deviation float64) Unstable {
	if deviation < 0 {
		deviation = 0
	}
	if deviation > 1 {
		deviation = 1
	}
	return Unstable{
		deviation: deviation,
		r:         rand.New(rand.NewSource(time.Now().UnixNano())),
		lock:      new(sync.Mutex),
	}
}

// AroundDuration returns a random duration within +/- deviation of base.
func (u Unstable) AroundDuration(base time.Duration) time.Duration {
	u.lock.Lock()
	val := time.Duration((1 + u.deviation - 2*u.deviation*u.r.Float64()) * float64(base))
	u.lock.Unlock()
	return val
}

// AroundInt returns a random int64 within +/- deviation of base.
func (u Unstable) AroundInt(base int64) int64 {
	u.lock.Lock()
	val := int64((1 + u.deviation - 2*u.deviation*u.r.Float64()) * float64(base))
	u.lock.Unlock()
	return val
}
