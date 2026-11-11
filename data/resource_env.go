package data

import (
	"strings"
	"time"

	"github.com/renniemaharaj/go-api-kit/internal/environment"
)

// NewResourceFromEnv loads cache settings, then builds the typed resource.
// NAME_CACHE_* settings override RESOURCE_CACHE_* defaults for that resource.
func NewResourceFromEnv[T any, ID comparable](config ResourceConfig[T, ID]) (*Resource[T, ID], error) {
	values, err := environment.Load(".env")
	if err != nil {
		return nil, err
	}

	prefix := strings.ToUpper(strings.ReplaceAll(config.Name, "-", "_"))
	ttl := values.Duration("RESOURCE_CACHE_TTL", time.Minute)
	maxEntries := values.Int("RESOURCE_CACHE_MAX_ENTRIES", 1024)
	config.Cache = CachePolicy{
		TTL:        values.Duration(prefix+"_CACHE_TTL", ttl),
		MaxEntries: values.Int(prefix+"_CACHE_MAX_ENTRIES", maxEntries),
	}
	if err := values.Err(); err != nil {
		return nil, err
	}

	return NewResource(config)
}
