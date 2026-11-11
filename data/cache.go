// Package data provides optional caching for context-aware loading functions.
package data

import (
	"context"
	"fmt"
	"time"

	"github.com/renniemaharaj/go-api-kit/internal/cache"
)

// CacheConfig enables caching with a positive TTL. MaxEntries defaults to 1024.
type CacheConfig[T any] struct {
	TTL        time.Duration
	MaxEntries int
	// Scope partitions data by tenant or caller attributes that affect loading.
	Scope func(context.Context) (string, error)
	// Clone must deep-copy mutable values; omit only for immutable cached data.
	Clone func(T) T
}

// Cache wraps a loading function without prescribing storage or write operations.
// Its callbacks must be safe for concurrent use. Concurrent misses are not coalesced.
type Cache[T any, ID comparable] struct {
	load    func(context.Context, ID) (T, error)
	entries *cache.Cache[T, ID]
	scope   func(context.Context) (string, error)
	clone   func(T) T
}

func NewCache[T any, ID comparable](load func(context.Context, ID) (T, error), config CacheConfig[T]) (*Cache[T, ID], error) {
	if load == nil {
		return nil, fmt.Errorf("data: load function is required")
	}

	if config.TTL < 0 || config.MaxEntries < 0 {
		return nil, fmt.Errorf("data: cache TTL and MaxEntries cannot be negative")
	}

	result := &Cache[T, ID]{load: load, scope: config.Scope, clone: config.Clone}
	if config.TTL > 0 {
		result.entries = cache.New[T, ID](config.TTL, config.MaxEntries)
	}

	return result, nil
}

// Get caches successful loads. Errors and canceled loads are never cached.
func (c *Cache[T, ID]) Get(ctx context.Context, id ID) (T, error) {
	var zero T
	scope, err := c.cacheScope(ctx)
	if err != nil {
		return zero, err
	}

	key := cache.Key[ID]{Scope: scope, ID: id}
	var generation uint64
	if c.entries != nil {
		value, found, current := c.entries.Get(key)
		if found {
			value = c.copy(value)
			if err := ctx.Err(); err != nil {
				return zero, err
			}

			return value, nil
		}

		generation = current
	}

	value, err := c.load(ctx, id)
	if err != nil {
		return zero, err
	}

	if err := ctx.Err(); err != nil {
		return zero, err
	}

	if c.entries != nil {
		cloned := c.copy(value)
		if err := ctx.Err(); err != nil {
			return zero, err
		}

		c.entries.Set(key, cloned, generation)
	}

	return value, nil
}

// Invalidate removes IDs in the current scope and fences earlier loads.
// After a commit, use context.WithoutCancel(ctx) to finish invalidation.
func (c *Cache[T, ID]) Invalidate(ctx context.Context, ids ...ID) error {
	scope, err := c.cacheScope(ctx)
	if err != nil {
		return err
	}

	if c.entries != nil && len(ids) > 0 {
		c.entries.Invalidate(scope, ids...)
	}

	return nil
}

// InvalidateAll clears the current scope and fences earlier in-flight loads.
func (c *Cache[T, ID]) InvalidateAll(ctx context.Context) error {
	scope, err := c.cacheScope(ctx)
	if err != nil {
		return err
	}

	if c.entries != nil {
		c.entries.InvalidateAll(scope)
	}

	return nil
}

func (c *Cache[T, ID]) cacheScope(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	if c.scope == nil {
		return "", nil
	}

	scope, err := c.scope(ctx)
	if err != nil {
		return "", err
	}

	return scope, ctx.Err()
}

func (c *Cache[T, ID]) copy(value T) T {
	if c.clone == nil {
		return value
	}

	return c.clone(value)
}
