package cache

import (
	"sync"
	"time"
)

// Cache holds cached data with an expiration time
type Cache[T any] struct {
    data      *T           // Use pointer to T to check for nil
    expiresAt time.Time
    mu        sync.RWMutex // Protects concurrent access
}

// New creates a new Cache instance
func New[T any]() *Cache[T] {
    return &Cache[T]{}
}

// Get retrieves the cached data if it’s not expired
func (c *Cache[T]) Get() (T, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()

    var zero T
    if c.data == nil || time.Now().After(c.expiresAt) {
        return zero, false
    }
    return *c.data, true
}

// Set stores data in the cache with an expiration duration
func (c *Cache[T]) Set(data T, ttl time.Duration) {
    c.mu.Lock()
    defer c.mu.Unlock()

    c.data = &data
    c.expiresAt = time.Now().Add(ttl)
}
