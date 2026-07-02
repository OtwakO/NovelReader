package analyzer

import (
	"container/list"
	"sync"
)

// CacheManager is an in-memory LRU cache for analyzer results.
// Bounded by maxEntries — prevents unbounded memory growth under multi-user load.
type CacheManager struct {
	mu         sync.Mutex
	data       map[string]*list.Element
	order      *list.List
	maxEntries int
}

type cacheEntry struct {
	key   string
	value string
}

const defaultMaxEntries = 4096

func NewCacheManager() *CacheManager {
	return &CacheManager{
		data:       make(map[string]*list.Element),
		order:      list.New(),
		maxEntries: defaultMaxEntries,
	}
}

func (c *CacheManager) Get(key string) string {
	c.mu.Lock()
	defer c.mu.Unlock()

	if el, ok := c.data[key]; ok {
		c.order.MoveToFront(el)
		return el.Value.(*cacheEntry).value
	}
	return ""
}

func (c *CacheManager) Put(key, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Update existing
	if el, ok := c.data[key]; ok {
		c.order.MoveToFront(el)
		el.Value.(*cacheEntry).value = value
		return
	}

	// Evict oldest if at capacity
	if c.order.Len() >= c.maxEntries {
		oldest := c.order.Back()
		if oldest != nil {
			c.order.Remove(oldest)
			delete(c.data, oldest.Value.(*cacheEntry).key)
		}
	}

	// Insert new
	entry := &cacheEntry{key: key, value: value}
	el := c.order.PushFront(entry)
	c.data[key] = el
}

func (c *CacheManager) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.data[key]; ok {
		c.order.Remove(el)
		delete(c.data, key)
	}
}
