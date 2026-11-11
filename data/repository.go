// Package data manages resource storage, service rules, caching, and HTTP wiring.
package data

import (
	"context"
	"errors"
)

var (
	// ErrNotFound can be returned by a repository when a record does not exist.
	ErrNotFound = errors.New("record not found")

	// ErrUnsupported indicates that an optional repository operation is absent.
	ErrUnsupported = errors.New("operation not supported")
)

// Repository is the storage contract used by a Resource and custom services.
// Implementations must be safe for concurrent use and honor request contexts.
type Repository[T any, ID comparable] interface {
	Get(context.Context, ID) (T, error)
	Save(context.Context, T) (T, error)
	Delete(context.Context, ID) error
}
