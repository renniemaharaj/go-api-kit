package cache

import (
	"container/list"
	"sync"
	"time"
)

type Key[ID comparable] struct {
	Scope string
	ID    ID
}

type cacheEntry[T any, ID comparable] struct {
	key       Key[ID]
	value     T
	expiresAt time.Time
}

type Cache[T any, ID comparable] struct {
	mu         sync.Mutex
	entries    map[Key[ID]]*list.Element
	recent     *list.List
	ttl        time.Duration
	maxEntries int
	generation uint64
	now        func() time.Time
}

func New[T any, ID comparable](ttl time.Duration, capacity int) *Cache[T, ID] {
	maxEntries := capacity
	if maxEntries == 0 {
		maxEntries = 1024
	}

	return &Cache[T, ID]{
		entries:    make(map[Key[ID]]*list.Element),
		recent:     list.New(),
		ttl:        ttl,
		maxEntries: maxEntries,
		now:        time.Now,
	}
}

func (c *Cache[T, ID]) Get(key Key[ID]) (T, bool, uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var zero T
	element, ok := c.entries[key]
	if !ok {
		return zero, false, c.generation
	}

	entry := element.Value.(cacheEntry[T, ID])
	if !c.now().Before(entry.expiresAt) {
		c.remove(element)
		return zero, false, c.generation
	}

	c.recent.MoveToFront(element)
	return entry.value, true, c.generation
}

func (c *Cache[T, ID]) Set(key Key[ID], value T, generation uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// An invalidation fences loads that started before the change.
	if c.generation != generation {
		return
	}

	entry := cacheEntry[T, ID]{
		key:       key,
		value:     value,
		expiresAt: c.now().Add(c.ttl),
	}

	if element, ok := c.entries[key]; ok {
		element.Value = entry
		c.recent.MoveToFront(element)
		return
	}

	c.entries[key] = c.recent.PushFront(entry)
	if c.recent.Len() > c.maxEntries {
		c.remove(c.recent.Back())
	}
}

func (c *Cache[T, ID]) Invalidate(scope string, ids ...ID) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.generation++
	for _, id := range ids {
		key := Key[ID]{
			Scope: scope,
			ID:    id,
		}
		if element, ok := c.entries[key]; ok {
			c.remove(element)
		}
	}
}

func (c *Cache[T, ID]) InvalidateAll(scope string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.generation++
	for key, element := range c.entries {
		if key.Scope == scope {
			c.remove(element)
		}
	}
}

func (c *Cache[T, ID]) remove(element *list.Element) {
	entry := element.Value.(cacheEntry[T, ID])
	delete(c.entries, entry.key)
	c.recent.Remove(element)
}
