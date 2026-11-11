package kit

import (
	"context"
	"io/fs"
	"net/http"
	"time"

	"github.com/renniemaharaj/go-api-kit/internal/environment"
	"github.com/renniemaharaj/go-api-kit/internal/files"
)

// FileCacheConfig bounds cached file contents by entry count and file size.
// A positive TTL enables caching; zero streams files directly from storage.
type FileCacheConfig struct {
	TTL          time.Duration
	MaxEntries   int
	MaxFileBytes int64
}

// FileServer serves cached file contents with a separate reader per request.
// Authentication and rate limiting must wrap this handler, including cache hits.
type FileServer struct {
	store *files.Store
}

func NewFileServer(fsys fs.FS, config FileCacheConfig) (*FileServer, error) {
	store, err := files.New(fsys, files.Config{
		TTL:          config.TTL,
		MaxEntries:   config.MaxEntries,
		MaxFileBytes: config.MaxFileBytes,
	})
	if err != nil {
		return nil, err
	}

	return &FileServer{store: store}, nil
}

func (s *FileServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if r.Context().Err() != nil {
		http.Error(w, "request canceled", http.StatusServiceUnavailable)
		return
	}

	// Browsers must revalidate; the server's content cache is independent.
	w.Header().Set("Cache-Control", "private, no-cache")

	fsys := s.store.FileSystem(r.Context(), w.Header())
	http.FileServerFS(fsys).ServeHTTP(w, r)
}

// Invalidate removes named files; use index.html or / for the root document.
func (s *FileServer) Invalidate(ctx context.Context, paths ...string) error {
	return s.store.Invalidate(ctx, paths...)
}

// InvalidateAll removes all cached file contents.
func (s *FileServer) InvalidateAll(ctx context.Context) error {
	return s.store.InvalidateAll(ctx)
}

// NewFileServerFromEnv loads file-cache limits from .env and process variables.
func NewFileServerFromEnv(fsys fs.FS) (*FileServer, error) {
	values, err := environment.Load(".env")
	if err != nil {
		return nil, err
	}

	config := FileCacheConfig{
		TTL:          values.Duration("FILE_CACHE_TTL", time.Minute),
		MaxEntries:   values.Int("FILE_CACHE_MAX_ENTRIES", 64),
		MaxFileBytes: values.Int64("FILE_CACHE_MAX_BYTES", 1<<20),
	}
	if err := values.Err(); err != nil {
		return nil, err
	}

	return NewFileServer(fsys, config)
}
