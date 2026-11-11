package repository

import "context"

type Store[T any, ID comparable] interface {
	Get(context.Context, ID) (T, error)
	Save(context.Context, T) (T, error)
	Delete(context.Context, ID) error
}

type Functions[T any, ID comparable] struct {
	Load        func(context.Context, ID) (T, error)
	Store       func(context.Context, T) (T, error)
	Remove      func(context.Context, ID) error
	Unsupported error
}

func (r Functions[T, ID]) Get(ctx context.Context, id ID) (T, error) {
	var zero T
	if err := ctx.Err(); err != nil {
		return zero, err
	}

	return r.Load(ctx, id)
}

func (r Functions[T, ID]) Save(ctx context.Context, value T) (T, error) {
	var zero T
	if err := ctx.Err(); err != nil {
		return zero, err
	}

	if r.Store == nil {
		return zero, r.Unsupported
	}

	return r.Store(ctx, value)
}

func (r Functions[T, ID]) Delete(ctx context.Context, id ID) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if r.Remove == nil {
		return r.Unsupported
	}

	return r.Remove(ctx, id)
}
