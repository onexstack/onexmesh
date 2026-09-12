// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package resilience

import (
	"context"
	"log/slog"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/onexstack/onexmesh/pkg/core/collection"
	"github.com/onexstack/onexmesh/pkg/core/stat"
	"github.com/onexstack/onexmesh/pkg/errno"
)

// ErrServiceOverloaded is returned when the adaptive load shedder drops a
// request because the service is overloaded.
var ErrServiceOverloaded = errno.ErrServiceOverloaded

const (
	defaultShedderBuckets      = 50
	defaultShedderWindow       = 5 * time.Second
	defaultShedderCpuThreshold = 900 // per-mille, i.e. 90%
	defaultShedderMinRt        = float64(time.Second / time.Millisecond)

	flyingBeta               = 0.9
	coolOffDuration          = time.Second
	cpuMax                   = 1000
	millisecondsPerSecond    = 1000
	overloadFactorLowerBound = 0.1
)

// systemOverloadChecker is a variable so tests can simulate overload
// deterministically without depending on real CPU sampling.
var systemOverloadChecker = func(cpuThreshold int64) bool {
	return stat.CpuUsage() >= cpuThreshold
}

// ShedderOption configures the adaptive load shedder.
type ShedderOption func(*shedderOptions)

type shedderOptions struct {
	window       time.Duration
	buckets      int
	cpuThreshold int64
}

// WithShedderWindow sets the rolling window duration. Default 5s.
func WithShedderWindow(window time.Duration) ShedderOption {
	return func(o *shedderOptions) { o.window = window }
}

// WithShedderBuckets sets the number of buckets in the window. Default 50.
func WithShedderBuckets(buckets int) ShedderOption {
	return func(o *shedderOptions) { o.buckets = buckets }
}

// WithShedderCpuThreshold sets the CPU threshold (per-mille, 0-1000) above which
// the shedder starts dropping. Default 900.
func WithShedderCpuThreshold(threshold int64) ShedderOption {
	return func(o *shedderOptions) { o.cpuThreshold = threshold }
}

// Shedder returns a middleware that adaptively drops requests when the service
// is overloaded, based on the smoothed CPU usage and the derived maximum
// in-flight requests (Little's law). It is an inbound overload-protection
// pattern; outbound resilience uses Breaker/Retry instead.
func Shedder(opts ...ShedderOption) Middleware {
	s := newAdaptiveShedder(opts...)
	return func(next Handler) Handler {
		return func(ctx context.Context) (err error) {
			promise, allowErr := s.Allow()
			if allowErr != nil {
				return allowErr
			}
			defer func() {
				if err != nil {
					promise.Fail()
				} else {
					promise.Pass()
				}
			}()
			return next(ctx)
		}
	}
}

// promise reports the outcome of a request admitted by the shedder.
type promise struct {
	start   time.Time
	shedder *adaptiveShedder
}

func (p *promise) Pass() {
	rt := float64(time.Since(p.start)) / float64(time.Millisecond)
	p.shedder.addFlying(-1)
	p.shedder.rtCounter.Add(int64(math.Ceil(rt)))
	p.shedder.passCounter.Add(1)
}

func (p *promise) Fail() {
	p.shedder.addFlying(-1)
}

// adaptiveShedder is the default Shedder implementation.
type adaptiveShedder struct {
	cpuThreshold int64
	windowScale  float64
	flying       int64
	avgFlying    float64

	avgFlyingLock   sync.Mutex
	overloadTime    atomic.Int64
	droppedRecently atomic.Bool
	passCounter     *collection.RollingWindow[int64, *collection.Bucket[int64]]
	rtCounter       *collection.RollingWindow[int64, *collection.Bucket[int64]]
}

func newAdaptiveShedder(opts ...ShedderOption) *adaptiveShedder {
	options := shedderOptions{
		window:       defaultShedderWindow,
		buckets:      defaultShedderBuckets,
		cpuThreshold: defaultShedderCpuThreshold,
	}
	for _, opt := range opts {
		opt(&options)
	}

	bucketDuration := options.window / time.Duration(options.buckets)
	newBucket := func() *collection.Bucket[int64] { return &collection.Bucket[int64]{} }
	return &adaptiveShedder{
		cpuThreshold: options.cpuThreshold,
		windowScale:  float64(time.Second) / float64(bucketDuration) / millisecondsPerSecond,
		passCounter:  collection.NewRollingWindow(newBucket, options.buckets, bucketDuration, collection.IgnoreCurrentBucket[int64, *collection.Bucket[int64]]()),
		rtCounter:    collection.NewRollingWindow(newBucket, options.buckets, bucketDuration, collection.IgnoreCurrentBucket[int64, *collection.Bucket[int64]]()),
	}
}

// Allow admits a request, returning a promise to report its outcome, or
// ErrServiceOverloaded when the service is overloaded.
func (as *adaptiveShedder) Allow() (promise, error) {
	if as.shouldDrop() {
		as.droppedRecently.Store(true)
		return promise{}, ErrServiceOverloaded
	}

	as.addFlying(1)
	return promise{start: time.Now(), shedder: as}, nil
}

func (as *adaptiveShedder) addFlying(delta int64) {
	flying := atomic.AddInt64(&as.flying, delta)
	// Update avgFlying only when a request finishes so it lags behind flying and
	// changes more smoothly: it accepts more on spikes and fewer on drops.
	if delta < 0 {
		as.avgFlyingLock.Lock()
		as.avgFlying = as.avgFlying*flyingBeta + float64(flying)*(1-flyingBeta)
		as.avgFlyingLock.Unlock()
	}
}

func (as *adaptiveShedder) highThru() bool {
	as.avgFlyingLock.Lock()
	avgFlying := as.avgFlying
	as.avgFlyingLock.Unlock()
	maxFlight := as.maxFlight() * as.overloadFactor()
	return avgFlying > maxFlight && float64(atomic.LoadInt64(&as.flying)) > maxFlight
}

func (as *adaptiveShedder) maxFlight() float64 {
	// maxFlight = maxPass * minRt * windowScale (Little's law).
	maxFlight := float64(as.maxPass()) * as.minRt() * as.windowScale
	return atLeast(maxFlight, 1)
}

func (as *adaptiveShedder) maxPass() int64 {
	var result int64 = 1
	as.passCounter.Reduce(func(b *collection.Bucket[int64]) {
		if b.Sum > result {
			result = b.Sum
		}
	})
	return result
}

func (as *adaptiveShedder) minRt() float64 {
	result := defaultShedderMinRt
	as.rtCounter.Reduce(func(b *collection.Bucket[int64]) {
		if b.Count <= 0 {
			return
		}
		avg := math.Round(float64(b.Sum) / float64(b.Count))
		if avg < result {
			result = avg
		}
	})
	return result
}

func (as *adaptiveShedder) overloadFactor() float64 {
	factor := (cpuMax - float64(stat.CpuUsage())) / (cpuMax - float64(as.cpuThreshold))
	// Always accept at least 10% of requests, even under heavy overload.
	return between(factor, overloadFactorLowerBound, 1)
}

func (as *adaptiveShedder) shouldDrop() bool {
	if as.systemOverloaded() || as.stillHot() {
		if as.highThru() {
			flying := atomic.LoadInt64(&as.flying)
			as.avgFlyingLock.Lock()
			avgFlying := as.avgFlying
			as.avgFlyingLock.Unlock()
			slog.Error("shedder dropped request",
				"cpu", stat.CpuUsage(),
				"max_pass", as.maxPass(),
				"min_rt", as.minRt(),
				"hot", as.stillHot(),
				"flying", flying,
				"avg_flying", avgFlying,
			)
			return true
		}
	}
	return false
}

func (as *adaptiveShedder) stillHot() bool {
	if !as.droppedRecently.Load() {
		return false
	}

	overloadTime := as.overloadTime.Load()
	if overloadTime == 0 {
		return false
	}

	if time.Since(time.Unix(0, overloadTime)) < coolOffDuration {
		return true
	}

	as.droppedRecently.Store(false)
	return false
}

func (as *adaptiveShedder) systemOverloaded() bool {
	if !systemOverloadChecker(as.cpuThreshold) {
		return false
	}
	as.overloadTime.Store(time.Now().UnixNano())
	return true
}

// between clamps x to the range [lower, upper].
func between(x, lower, upper float64) float64 {
	if x < lower {
		return lower
	}
	if x > upper {
		return upper
	}
	return x
}
