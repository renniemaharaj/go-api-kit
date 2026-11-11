package kit

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"
)

type countingFS struct {
	fsys  fs.FS
	mu    sync.Mutex
	opens map[string]int
}

func (f *countingFS) Open(name string) (fs.File, error) {
	f.mu.Lock()
	f.opens[name]++
	f.mu.Unlock()
	return f.fsys.Open(name)
}

func fileRequest(handler http.Handler, method, target string, headers map[string]string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, target, nil)
	for key, value := range headers {
		request.Header.Set(key, value)
	}

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func TestFileServerCachesContentsAcrossQueryStrings(t *testing.T) {
	files := fstest.MapFS{
		"app.js": &fstest.MapFile{Data: []byte("original")},
	}
	storage := &countingFS{
		fsys:  files,
		opens: make(map[string]int),
	}
	server, err := NewFileServer(storage, FileCacheConfig{TTL: time.Hour})
	if err != nil {
		t.Fatal(err)
	}

	first := fileRequest(server, http.MethodGet, "/app.js", nil)
	if first.Code != http.StatusOK || first.Body.String() != "original" {
		t.Fatalf("got %d %q", first.Code, first.Body.String())
	}

	if first.Header().Get("ETag") == "" {
		t.Fatal("cached file has no ETag")
	}

	files["app.js"] = &fstest.MapFile{Data: []byte("replacement")}
	for index := range 20 {
		response := fileRequest(server, http.MethodGet, fmt.Sprintf("/app.js?random=%d", index), nil)
		if response.Code != http.StatusOK || response.Body.String() != "original" {
			t.Fatalf("query string bypassed cache: %d %q", response.Code, response.Body.String())
		}
	}

	if storage.opens["app.js"] != 1 {
		t.Fatalf("file opened %d times, want 1", storage.opens["app.js"])
	}

	if err := server.Invalidate(context.Background(), "/app.js"); err != nil {
		t.Fatal(err)
	}

	response := fileRequest(server, http.MethodGet, "/app.js?random=again", nil)
	if response.Code != http.StatusOK || response.Body.String() != "replacement" {
		t.Fatalf("invalidation did not refresh content: %d %q", response.Code, response.Body.String())
	}

	if response.Header().Get("ETag") == first.Header().Get("ETag") {
		t.Fatal("changed contents retained the old ETag")
	}
}

func TestFileServerExpirationAndDisabledCaching(t *testing.T) {
	for _, ttl := range []time.Duration{
		0,
		10 * time.Millisecond,
	} {
		t.Run(ttl.String(), func(t *testing.T) {
			files := fstest.MapFS{
				"file.txt": &fstest.MapFile{Data: []byte("first")},
			}
			server, err := NewFileServer(files, FileCacheConfig{TTL: ttl})
			if err != nil {
				t.Fatal(err)
			}

			first := fileRequest(server, http.MethodGet, "/file.txt", nil)
			if first.Code != http.StatusOK || first.Body.String() != "first" {
				t.Fatalf("first load: %d %q", first.Code, first.Body.String())
			}

			files["file.txt"] = &fstest.MapFile{Data: []byte("second")}
			if ttl > 0 {
				time.Sleep(3 * ttl)
			}

			second := fileRequest(server, http.MethodGet, "/file.txt", nil)
			if second.Code != http.StatusOK || second.Body.String() != "second" {
				t.Fatalf("did not refresh: %d %q", second.Code, second.Body.String())
			}
		})
	}
}

func TestFileServerPreservesConditionalRequestsAndRanges(t *testing.T) {
	server, err := NewFileServer(fstest.MapFS{
		"file.txt": &fstest.MapFile{Data: []byte("abcdef")},
	}, FileCacheConfig{TTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}

	initial := fileRequest(server, http.MethodGet, "/file.txt", nil)

	conditional := fileRequest(server, http.MethodGet, "/file.txt", map[string]string{
		"If-None-Match": initial.Header().Get("ETag"),
	})
	if conditional.Code != http.StatusNotModified || conditional.Body.Len() != 0 {
		t.Fatalf("conditional response: %d %q", conditional.Code, conditional.Body.String())
	}

	ranged := fileRequest(server, http.MethodGet, "/file.txt", map[string]string{
		"Range": "bytes=1-3",
	})
	if ranged.Code != http.StatusPartialContent || ranged.Body.String() != "bcd" {
		t.Fatalf("range response: %d %q", ranged.Code, ranged.Body.String())
	}

	head := fileRequest(server, http.MethodHead, "/file.txt", nil)
	if head.Code != http.StatusOK || head.Body.Len() != 0 || head.Header().Get("Content-Length") != "6" {
		t.Fatalf("HEAD response: %d %v", head.Code, head.Header())
	}

	post := fileRequest(server, http.MethodPost, "/file.txt", nil)
	if post.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST response: %d", post.Code)
	}
}

func TestFileServerStreamsOversizedFilesAndDoesNotCacheErrors(t *testing.T) {
	files := fstest.MapFS{
		"large.txt": &fstest.MapFile{Data: []byte("larger than the limit")},
	}
	server, err := NewFileServer(files, FileCacheConfig{
		TTL:          time.Minute,
		MaxFileBytes: 4,
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, body := range []string{
		"larger than the limit",
		"new contents are still large",
	} {
		files["large.txt"] = &fstest.MapFile{Data: []byte(body)}
		response := fileRequest(server, http.MethodGet, "/large.txt", nil)
		if response.Code != http.StatusOK || response.Body.String() != body {
			t.Fatalf("oversized file: %d %q", response.Code, response.Body.String())
		}

		if response.Header().Get("ETag") != "" {
			t.Fatal("oversized file was cached")
		}
	}

	missing := fileRequest(server, http.MethodGet, "/new.txt", nil)
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing file: %d", missing.Code)
	}

	files["new.txt"] = &fstest.MapFile{Data: []byte("new")}
	created := fileRequest(server, http.MethodGet, "/new.txt", nil)
	if created.Code != http.StatusOK || created.Body.String() != "new" {
		t.Fatalf("error was cached: %d %q", created.Code, created.Body.String())
	}
}

func TestFileServerContextAndInvalidationValidation(t *testing.T) {
	storage := &countingFS{
		fsys:  fstest.MapFS{},
		opens: make(map[string]int),
	}
	server, err := NewFileServer(storage, FileCacheConfig{TTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	request := httptest.NewRequest(http.MethodGet, "/file.txt", nil).WithContext(ctx)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable || len(storage.opens) != 0 {
		t.Fatalf("canceled request accessed storage: %d, %v", response.Code, storage.opens)
	}

	if err := server.Invalidate(ctx, "file.txt"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Invalidate lost cancellation: %v", err)
	}

	if err := server.Invalidate(context.Background(), "../secret"); !errors.Is(err, fs.ErrInvalid) {
		t.Fatalf("Invalidate accepted traversal: %v", err)
	}

	if _, err := NewFileServer(storage, FileCacheConfig{MaxFileBytes: -1}); err == nil {
		t.Fatal("negative file limit was accepted")
	}
}

func TestFileServerConcurrentReadersUseIndependentPositions(t *testing.T) {
	contents := strings.Repeat("independent reader", 1024)
	server, err := NewFileServer(fstest.MapFS{
		"file.txt": &fstest.MapFile{Data: []byte(contents)},
	}, FileCacheConfig{TTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}

	fileRequest(server, http.MethodGet, "/file.txt", nil)

	var readers sync.WaitGroup
	for range 20 {
		readers.Go(func() {
			response := fileRequest(server, http.MethodGet, "/file.txt", nil)
			if response.Code != http.StatusOK || response.Body.String() != contents {
				t.Errorf("shared reader position: status=%d bytes=%d", response.Code, response.Body.Len())
			}
		})
	}

	readers.Wait()
}
