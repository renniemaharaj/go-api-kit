package repository

import (
	"context"
	"time"

	"github.com/renniemaharaj/go-api-kit/internal/cache"
)

type CacheConfig[T any, ID comparable] struct {
	TTL         time.Duration
	MaxEntries  int
	Key         func(T) ID
	Scope       func(context.Context) (string, error)
	Clone       func(T) T
	Unsupported error
}

type Cached[T any, ID comparable] struct {
	next        Store[T, ID]
	cache       *cache.Cache[T, ID]
	key         func(T) ID
	scope       func(context.Context) (string, error)
	clone       func(T) T
	unsupported error
}

func NewCached[T any, ID comparable](next Store[T, ID], config CacheConfig[T, ID]) *Cached[T, ID] {
	return &Cached[T, ID]{
		next:        next,
		cache:       cache.New[T, ID](config.TTL, config.MaxEntries),
		key:         config.Key,
		scope:       config.Scope,
		clone:       config.Clone,
		unsupported: config.Unsupported,
	}
}

func (r *Cached[T, ID]) Get(ctx context.Context, id ID) (T, error) {
	var zero T
	scope, err := r.cacheScope(ctx)
	if err != nil {
		return zero, err
	}

	key := cache.Key[ID]{
		Scope: scope,
		ID:    id,
	}
	value, found, generation := r.cache.Get(key)
	if found {
		return r.copy(value), nil
	}

	value, err = r.next.Get(ctx, id)
	if err != nil {
		return zero, err
	}

	if err := ctx.Err(); err != nil {
		return zero, err
	}

	r.cache.Set(key, r.copy(value), generation)
	return value, nil
}

func (r *Cached[T, ID]) Save(ctx context.Context, value T) (T, error) {
	var zero T
	scope, err := r.cacheScope(ctx)
	if err != nil {
		return zero, err
	}

	if r.key == nil {
		return zero, r.unsupported
	}

	// Capture the original ID before storage can change a pointer-backed value.
	originalID := r.key(value)
	saved, err := r.next.Save(ctx, value)
	if err != nil {
		return zero, err
	}

	// A successful commit still needs invalidation if its context was canceled.
	r.cache.Invalidate(scope, originalID, r.key(saved))
	return saved, nil
}

func (r *Cached[T, ID]) Delete(ctx context.Context, id ID) error {
	scope, err := r.cacheScope(ctx)
	if err != nil {
		return err
	}

	if err := r.next.Delete(ctx, id); err != nil {
		return err
	}

	r.cache.Invalidate(scope, id)
	return nil
}

func (r *Cached[T, ID]) cacheScope(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	if r.scope == nil {
		return "", nil
	}

	scope, err := r.scope(ctx)
	if err != nil {
		return "", err
	}

	return scope, ctx.Err()
}

func (r *Cached[T, ID]) copy(value T) T {
	if r.clone == nil {
		return value
	}

	return r.clone(value)
}

func (r *Cached[T, ID]) Invalidate(ctx context.Context, ids ...ID) error {
	scope, err := r.cacheScope(ctx)
	if err != nil {
		return err
	}

	r.cache.Invalidate(scope, ids...)
	return nil
}

func (r *Cached[T, ID]) InvalidateAll(ctx context.Context) error {
	scope, err := r.cacheScope(ctx)
	if err != nil {
		return err
	}

	r.cache.InvalidateAll(scope)
	return nil
}
