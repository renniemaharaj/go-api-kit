package files

import (
	"context"
	"fmt"
	"io/fs"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/renniemaharaj/go-api-kit/internal/repository"
)

// Config bounds cached file contents by entry count and file size.
// A positive TTL enables caching; zero streams files directly from storage.
type Config struct {
	TTL          time.Duration
	MaxEntries   int
	MaxFileBytes int64
}

// Store caches immutable file contents and creates a reader per request.
type Store struct {
	repository   *fileRepository
	files        *repository.Cached[fileSnapshot, string]
	cacheEnabled bool
}

func New(fsys fs.FS, config Config) (*Store, error) {
	if fsys == nil {
		return nil, fmt.Errorf("kit: file system is required")
	}

	if config.TTL < 0 || config.MaxEntries < 0 {
		return nil, fmt.Errorf("files: cache TTL and MaxEntries cannot be negative")
	}

	if config.MaxFileBytes < 0 || config.MaxFileBytes == math.MaxInt64 {
		return nil, fmt.Errorf("kit: MaxFileBytes must be between zero and MaxInt64-1")
	}

	if config.MaxEntries == 0 {
		config.MaxEntries = 64
	}

	if config.MaxFileBytes == 0 {
		config.MaxFileBytes = 1 << 20
	}

	storage := &fileRepository{
		fsys:         fsys,
		maxFileBytes: config.MaxFileBytes,
	}
	files := repository.NewCached(storage, repository.CacheConfig[fileSnapshot, string]{
		TTL:         config.TTL,
		MaxEntries:  config.MaxEntries,
		Unsupported: errReadOnly,
	})

	return &Store{
		repository:   storage,
		files:        files,
		cacheEnabled: config.TTL > 0,
	}, nil
}

func (s *Store) FileSystem(ctx context.Context, header http.Header) fs.FS {
	return requestFileSystem{
		store:  s,
		ctx:    ctx,
		header: header,
	}
}

// Invalidate removes named files; use index.html or / for the root document.
func (s *Store) Invalidate(ctx context.Context, paths ...string) error {
	names := make([]string, 0, len(paths))
	for _, path := range paths {
		name := strings.TrimPrefix(path, "/")
		if name == "" {
			name = "index.html"
		}

		if !fs.ValidPath(name) {
			return fmt.Errorf("kit: invalid file path %q: %w", path, fs.ErrInvalid)
		}

		names = append(names, name)
	}

	return s.files.Invalidate(ctx, names...)
}

func (s *Store) InvalidateAll(ctx context.Context) error {
	return s.files.InvalidateAll(ctx)
}
