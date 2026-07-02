package analyzer

import (
	"sync"
)

// CacheManager provides a simple in-memory cache for the analyzer.
// ponytail: naive map+mutex. Replace with LRU if memory becomes an issue.
type CacheManager struct {
	mu   sync.RWMutex
	data map[string]string
}

func NewCacheManager() *CacheManager {
	return &CacheManager{
		data: make(map[string]string),
	}
}

func (c *CacheManager) Get(key string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.data[key]
}

func (c *CacheManager) Put(key, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = value
}

func (c *CacheManager) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.data, key)
}
