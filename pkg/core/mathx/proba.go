// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package mathx

import (
	"math/rand"
	"sync"
	"time"
)

// Proba tests a boolean against a given probability, used by the circuit
// breaker's force-pass probe.
type Proba struct {
	r    *rand.Rand
	lock sync.Mutex
}

// NewProba returns a Proba.
func NewProba() *Proba {
	return &Proba{r: rand.New(rand.NewSource(time.Now().UnixNano()))}
}

// TrueOnProba reports true with probability proba.
func (p *Proba) TrueOnProba(proba float64) bool {
	p.lock.Lock()
	truth := p.r.Float64() < proba
	p.lock.Unlock()
	return truth
}
