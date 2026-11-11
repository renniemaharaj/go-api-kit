package data

import (
	"context"
	"fmt"

	"github.com/renniemaharaj/go-api-kit/internal/repository"
)

// Resource is a default service backed by a repository and optional read cache.
// It is safe for concurrent use when its callbacks and repository are too.
type Resource[T any, ID comparable] struct {
	name         string
	repository   Repository[T, ID]
	cached       *repository.Cached[T, ID]
	beforeGet    func(context.Context, ID) error
	beforeSave   func(context.Context, T) error
	beforeDelete func(context.Context, ID) error
}

// Get runs service rules before reading through the repository cache.
func (r *Resource[T, ID]) Get(ctx context.Context, id ID) (T, error) {
	var zero T
	if err := ctx.Err(); err != nil {
		return zero, r.operationError("get", err)
	}

	if r.beforeGet != nil {
		if err := r.beforeGet(ctx, id); err != nil {
			return zero, r.operationError("get", err)
		}
	}

	if err := ctx.Err(); err != nil {
		return zero, r.operationError("get", err)
	}

	value, err := r.repository.Get(ctx, id)
	if err != nil {
		return zero, r.operationError("get", err)
	}

	if err := ctx.Err(); err != nil {
		return zero, r.operationError("get", err)
	}

	return value, nil
}

// Save runs service rules and invalidates the input and saved IDs on success.
// The storage callback must return only after its transaction has committed.
func (r *Resource[T, ID]) Save(ctx context.Context, value T) (T, error) {
	var zero T
	if err := ctx.Err(); err != nil {
		return zero, r.operationError("save", err)
	}

	if r.beforeSave != nil {
		if err := r.beforeSave(ctx, value); err != nil {
			return zero, r.operationError("save", err)
		}
	}

	if err := ctx.Err(); err != nil {
		return zero, r.operationError("save", err)
	}

	saved, err := r.repository.Save(ctx, value)
	if err != nil {
		return zero, r.operationError("save", err)
	}

	return saved, nil
}

// Delete runs service rules and invalidates the ID after storage succeeds.
func (r *Resource[T, ID]) Delete(ctx context.Context, id ID) error {
	if err := ctx.Err(); err != nil {
		return r.operationError("delete", err)
	}

	if r.beforeDelete != nil {
		if err := r.beforeDelete(ctx, id); err != nil {
			return r.operationError("delete", err)
		}
	}

	if err := ctx.Err(); err != nil {
		return r.operationError("delete", err)
	}

	if err := r.repository.Delete(ctx, id); err != nil {
		return r.operationError("delete", err)
	}

	return nil
}

// Repository returns storage with caching and write invalidation intact.
// It bypasses service hooks; custom services must apply their own rules.
func (r *Resource[T, ID]) Repository() Repository[T, ID] {
	return r.repository
}

// Invalidate removes IDs in the current scope without calling storage.
// Reads started earlier cannot repopulate the invalidated cache.
func (r *Resource[T, ID]) Invalidate(ctx context.Context, ids ...ID) error {
	if err := ctx.Err(); err != nil {
		return r.operationError("invalidate", err)
	}

	if r.cached == nil || len(ids) == 0 {
		return nil
	}

	err := r.cached.Invalidate(ctx, ids...)
	if err != nil {
		return r.operationError("invalidate", err)
	}

	return nil
}

// InvalidateAll removes all entries in the current scope without calling storage.
// With no Scope function, this clears the entire resource cache.
func (r *Resource[T, ID]) InvalidateAll(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return r.operationError("invalidate all", err)
	}

	if r.cached == nil {
		return nil
	}

	err := r.cached.InvalidateAll(ctx)
	if err != nil {
		return r.operationError("invalidate all", err)
	}

	return nil
}

func (r *Resource[T, ID]) operationError(operation string, err error) error {
	return fmt.Errorf("%s: %s: %w", r.name, operation, err)
}
