package data

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/renniemaharaj/go-api-kit/internal/repository"
)

// CachePolicy enables a bounded, process-local cache when TTL is positive.
// A zero TTL disables caching. MaxEntries defaults to 1024 when enabled.
type CachePolicy struct {
	TTL        time.Duration
	MaxEntries int
}

// ResourceConfig connects a repository to service rules and optional caching.
// Hooks are service rules and run on every call, including cache hits.
type ResourceConfig[T any, ID comparable] struct {
	Name string
	Key  func(T) ID

	// Storage callbacks are an alternative to implementing Repository.
	Load   func(context.Context, ID) (T, error)
	Save   func(context.Context, T) (T, error)
	Delete func(context.Context, ID) error

	Repository Repository[T, ID]

	BeforeGet    func(context.Context, ID) error
	BeforeSave   func(context.Context, T) error
	BeforeDelete func(context.Context, ID) error

	Cache CachePolicy

	// Scope partitions cache entries and invalidation, for example by tenant.
	// Omit it only when each ID always identifies the same data across callers.
	Scope func(context.Context) (string, error)

	// Clone must deep-copy values that callers or storage may mutate.
	// Without Clone, cached pointers, maps, and slices must be treated as immutable.
	Clone func(T) T
}

// NewResource validates configuration and connects storage, caching, and hooks.
// Load is required for function storage; Save and Delete are optional.
func NewResource[T any, ID comparable](config ResourceConfig[T, ID]) (*Resource[T, ID], error) {
	if strings.TrimSpace(config.Name) == "" {
		return nil, fmt.Errorf("data: resource name is required")
	}

	if config.Cache.TTL < 0 || config.Cache.MaxEntries < 0 {
		return nil, fmt.Errorf("%s: cache TTL and MaxEntries cannot be negative", config.Name)
	}

	storage, err := config.storage()
	if err != nil {
		return nil, err
	}

	resource := &Resource[T, ID]{
		name:         config.Name,
		repository:   storage,
		beforeGet:    config.BeforeGet,
		beforeSave:   config.BeforeSave,
		beforeDelete: config.BeforeDelete,
	}

	if config.Cache.TTL > 0 {
		if config.Key == nil && (config.Save != nil || config.Repository != nil) {
			return nil, fmt.Errorf("%s: Key is required for cached saves", config.Name)
		}

		resource.cached = repository.NewCached(storage, repository.CacheConfig[T, ID]{
			TTL:         config.Cache.TTL,
			MaxEntries:  config.Cache.MaxEntries,
			Key:         config.Key,
			Scope:       config.Scope,
			Clone:       config.Clone,
			Unsupported: ErrUnsupported,
		})
		resource.repository = resource.cached
	}

	return resource, nil
}

func (config ResourceConfig[T, ID]) storage() (Repository[T, ID], error) {
	if config.Repository != nil {
		if config.Load != nil || config.Save != nil || config.Delete != nil {
			return nil, fmt.Errorf("%s: use either Repository or storage functions", config.Name)
		}

		return config.Repository, nil
	}

	if config.Load == nil {
		return nil, fmt.Errorf("%s: Load or Repository is required", config.Name)
	}

	return repository.Functions[T, ID]{
		Load:        config.Load,
		Store:       config.Save,
		Remove:      config.Delete,
		Unsupported: ErrUnsupported,
	}, nil
}
