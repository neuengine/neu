package asset

import "sync"

// resourceRecord tracks one sub-resource loaded by ContentManager.
type resourceRecord struct {
	val      any // the decoded value (type-erased)
	refCount int64
}

// ContentManager ref-counts sub-resources keyed by a content URL string.
// When a refcount drops to zero the record is evicted from the cache.
// All methods are goroutine-safe.
type ContentManager struct {
	mu     sync.Mutex
	loaded map[string]*resourceRecord
}

// NewContentManager returns an empty ContentManager.
func NewContentManager() *ContentManager {
	return &ContentManager{loaded: make(map[string]*resourceRecord)}
}

// Load returns the cached value for url and true, incrementing its refcount.
// Returns (nil, false) if the content has not been loaded or was evicted.
func (c *ContentManager) Load(url string) (any, bool) {
	c.mu.Lock()
	rec, ok := c.loaded[url]
	if ok {
		rec.refCount++
	}
	c.mu.Unlock()
	if !ok {
		return nil, false
	}
	return rec.val, true
}

// Store inserts or replaces the value for url with an initial refcount of 1.
// If the url is already tracked its value is updated and refcount stays the same.
func (c *ContentManager) Store(url string, val any) {
	c.mu.Lock()
	if rec, ok := c.loaded[url]; ok {
		rec.val = val
	} else {
		c.loaded[url] = &resourceRecord{val: val, refCount: 1}
	}
	c.mu.Unlock()
}

// Release decrements the refcount for url. When the count reaches zero the
// entry is evicted. Returns true if the entry was evicted.
func (c *ContentManager) Release(url string) bool {
	c.mu.Lock()
	rec, ok := c.loaded[url]
	if !ok {
		c.mu.Unlock()
		return false
	}
	rec.refCount--
	evicted := rec.refCount <= 0
	if evicted {
		delete(c.loaded, url)
	}
	c.mu.Unlock()
	return evicted
}

// Len returns the number of currently-tracked entries.
func (c *ContentManager) Len() int {
	c.mu.Lock()
	n := len(c.loaded)
	c.mu.Unlock()
	return n
}
