// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package collection

import (
	"container/list"
	"sync"
	"time"

	"github.com/onexstack/onexmesh/pkg/core/mathx"
	"github.com/onexstack/onexmesh/pkg/core/syncx"
)

const (
	// slots is the number of timing-wheel slots, i.e. the wheel granularity.
	slots = 300
	// expiryDeviation spreads expiry so many cached items do not expire at once.
	expiryDeviation = 0.05
)

var emptyLruCache = emptyLru{}

// CacheOption customizes a Cache.
type CacheOption func(*Cache)

// Cache is an in-memory cache with timing-wheel expiry, optional LRU eviction
// and singleflight-protected read-through.
type Cache struct {
	lock           sync.Mutex
	data           map[string]any
	expire         time.Duration
	timingWheel    *TimingWheel
	lruCache       lru
	barrier        syncx.SingleFlight
	unstableExpiry mathx.Unstable
}

// NewCache returns a Cache whose entries expire after expire. The expiry is
// jittered slightly to avoid thundering-herd expirations.
func NewCache(expire time.Duration, opts ...CacheOption) (*Cache, error) {
	cache := &Cache{
		data:           make(map[string]any),
		expire:         expire,
		lruCache:       emptyLruCache,
		barrier:        syncx.NewSingleFlight(),
		unstableExpiry: mathx.NewUnstable(expiryDeviation),
	}

	for _, opt := range opts {
		opt(cache)
	}

	timingWheel, err := NewTimingWheel(time.Second, slots, func(k, _ any) {
		key, ok := k.(string)
		if !ok {
			return
		}
		cache.Del(key)
	})
	if err != nil {
		return nil, err
	}

	cache.timingWheel = timingWheel
	return cache, nil
}

// Del deletes the item with the given key.
func (c *Cache) Del(key string) {
	c.lock.Lock()
	delete(c.data, key)
	c.lruCache.remove(key)
	c.lock.Unlock()

	// RemoveTimer is called outside the lock because it may block on the wheel's
	// command channel; the LRU keeps data integrity.
	c.timingWheel.RemoveTimer(key)
}

// Get returns the item with the given key.
func (c *Cache) Get(key string) (any, bool) {
	return c.doGet(key)
}

// Set stores value under key with the cache's default expiry.
func (c *Cache) Set(key string, value any) {
	c.SetWithExpire(key, value, c.expire)
}

// SetWithExpire stores value under key with the given expiry.
func (c *Cache) SetWithExpire(key string, value any, expire time.Duration) {
	c.lock.Lock()
	_, ok := c.data[key]
	c.data[key] = value
	c.lruCache.add(key)
	c.lock.Unlock()

	expiry := c.unstableExpiry.AroundDuration(expire)
	if ok {
		c.timingWheel.MoveTimer(key, expiry)
	} else {
		c.timingWheel.SetTimer(key, value, expiry)
	}
}

// Take returns the item under key, fetching and caching it when absent. The
// fetch is deduplicated across concurrent callers via singleflight.
func (c *Cache) Take(key string, fetch func() (any, error)) (any, error) {
	if val, ok := c.doGet(key); ok {
		return val, nil
	}

	val, err := c.barrier.Do(key, func() (any, error) {
		// Double-check: another caller may have populated the cache while this
		// caller waited for the singleflight.
		if val, ok := c.doGet(key); ok {
			return val, nil
		}

		v, e := fetch()
		if e != nil {
			return nil, e
		}
		c.Set(key, v)
		return v, nil
	})
	if err != nil {
		return nil, err
	}
	return val, nil
}

// Close stops the cache's timing wheel and releases its goroutine.
func (c *Cache) Close() {
	if c.timingWheel != nil {
		c.timingWheel.Stop()
	}
}

func (c *Cache) doGet(key string) (any, bool) {
	c.lock.Lock()
	defer c.lock.Unlock()

	value, ok := c.data[key]
	if ok {
		c.lruCache.add(key)
	}
	return value, ok
}

func (c *Cache) onEvict(key string) {
	// Caller holds c.lock.
	delete(c.data, key)
	c.timingWheel.RemoveTimer(key)
}

// WithLimit caps the number of cached items via LRU eviction.
func WithLimit(limit int) CacheOption {
	return func(cache *Cache) {
		if limit > 0 {
			cache.lruCache = newKeyLru(limit, cache.onEvict)
		}
	}
}

type (
	lru interface {
		add(key string)
		remove(key string)
	}

	emptyLru struct{}

	keyLru struct {
		limit    int
		evicts   *list.List
		elements map[string]*list.Element
		onEvict  func(key string)
	}
)

func (emptyLru) add(string)    {}
func (emptyLru) remove(string) {}

func newKeyLru(limit int, onEvict func(key string)) *keyLru {
	return &keyLru{
		limit:    limit,
		evicts:   list.New(),
		elements: make(map[string]*list.Element),
		onEvict:  onEvict,
	}
}

func (klru *keyLru) add(key string) {
	if elem, ok := klru.elements[key]; ok {
		klru.evicts.MoveToFront(elem)
		return
	}

	elem := klru.evicts.PushFront(key)
	klru.elements[key] = elem

	if klru.evicts.Len() > klru.limit {
		klru.removeOldest()
	}
}

func (klru *keyLru) remove(key string) {
	if elem, ok := klru.elements[key]; ok {
		klru.removeElement(elem)
	}
}

func (klru *keyLru) removeOldest() {
	if elem := klru.evicts.Back(); elem != nil {
		klru.removeElement(elem)
	}
}

func (klru *keyLru) removeElement(e *list.Element) {
	klru.evicts.Remove(e)
	key := e.Value.(string)
	delete(klru.elements, key)
	klru.onEvict(key)
}
