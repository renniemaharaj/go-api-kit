package v8

import (
	"context"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"testing/fstest"

	"github.com/renniemaharaj/go-api-kit/internal/filesystem"
	"github.com/renniemaharaj/go-api-kit/kit"
)

func testFileHandler(t *testing.T, fsys fs.FS) http.Handler {
	t.Helper()

	handler, err := NewHandler(fsys)
	if err != nil {
		t.Fatal(err)
	}

	return handler
}

func TestRunReturnsRootOpenError(t *testing.T) {
	t.Setenv("FRONTEND_DIR", filepath.Join(t.TempDir(), "missing"))
	err := Run()
	if err == nil {
		t.Fatal("expected an error for a missing frontend directory")
	}
}

func TestRunReturnsListenError(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	t.Setenv("FRONTEND_DIR", t.TempDir())
	t.Setenv("SERVER_ADDRESS", listener.Addr().String())
	if err := Run(); err == nil {
		t.Fatal("expected an error for an address that is already in use")
	}
}

func TestDirectoryHandlerServesFrontendFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}

	index := `<img src="/welcome.svg" alt="welcome">`
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte(index), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, "welcome.svg"), []byte("<svg>Welcome</svg>"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, "assets", "app.js"), []byte("app"), 0o644); err != nil {
		t.Fatal(err)
	}

	root, err := filesystem.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	handler := testFileHandler(t, root)

	for _, test := range []struct {
		path string
		body string
	}{
		{
			path: "/",
			body: index,
		},
		{
			path: "/welcome.svg",
			body: "<svg>Welcome</svg>",
		},
		{
			path: "/assets/app.js",
			body: "app",
		},
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, test.path, nil))
		if response.Code != http.StatusOK || response.Body.String() != test.body {
			t.Errorf("GET %s: got status %d and body %q", test.path, response.Code, response.Body.String())
		}
	}
}

func TestDefaultPageServesExternalStylesheetAndIcon(t *testing.T) {
	root, err := filesystem.OpenRoot(filepath.Join("..", "dist"))
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	handler := testFileHandler(t, root)

	page := httptest.NewRecorder()
	handler.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/", nil))
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), `id="copy-command"`) || !strings.Contains(page.Body.String(), `href="/assets/styles.css"`) {
		t.Fatalf("default page is missing interactive HTML or stylesheet link: status %d, body %q", page.Code, page.Body.String())
	}

	for _, asset := range []struct {
		path        string
		contentType string
	}{
		{
			path:        "/assets/styles.css",
			contentType: "text/css",
		},
		{
			path:        "/assets/v8.png",
			contentType: "image/png",
		},
		{
			path:        "/assets/welcome.svg",
			contentType: "image/svg+xml",
		},
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, asset.path, nil))
		if response.Code != http.StatusOK || !strings.HasPrefix(response.Header().Get("Content-Type"), asset.contentType) || response.Body.Len() == 0 {
			t.Errorf("GET %s: got status %d, content type %q, body length %d", asset.path, response.Code, response.Header().Get("Content-Type"), response.Body.Len())
		}
	}
}

func TestDirectoryHandlerRejectsEscapingSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation commonly requires extra privileges on Windows")
	}

	tempDir := t.TempDir()
	dir := filepath.Join(tempDir, "frontend")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	secret := filepath.Join(tempDir, "secret.txt")
	if err := os.WriteFile(secret, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.Symlink(secret, filepath.Join(dir, "public.txt")); err != nil {
		t.Fatal(err)
	}

	root, err := filesystem.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	handler := testFileHandler(t, root)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/public.txt", nil))
	if response.Code == http.StatusOK && response.Body.String() == "secret" {
		t.Fatalf("served file outside frontend directory: status %d, body %q", response.Code, response.Body.String())
	}
}

type observedFS struct {
	fs.FS
	opens atomic.Int64
}

func (f *observedFS) Open(name string) (fs.File, error) {
	f.opens.Add(1)
	return f.FS.Open(name)
}

func TestFileServerLimitsCacheHitsMissesAndChangingPaths(t *testing.T) {
	storage := &observedFS{
		FS: fstest.MapFS{
			"file.txt": &fstest.MapFile{Data: []byte("cached contents")},
		},
	}
	t.Setenv("RATE_LIMIT_REQUESTS", "1")
	t.Setenv("RATE_LIMIT_INTERVAL", "1h")
	t.Setenv("RATE_LIMIT_BURST", "3")
	t.Setenv("RATE_LIMIT_IDLE_TTL", "3h")
	handler, err := NewHandler(storage)
	if err != nil {
		t.Fatal(err)
	}

	for index, target := range []string{
		"/file.txt",
		"/file.txt?random=1",
		"/missing-42.txt",
		"/missing-57.txt?random=2",
		"/another-route",
	} {
		request := httptest.NewRequest(http.MethodGet, target, nil)
		request.RemoteAddr = fmt.Sprintf("192.0.2.1:%d", 1000+index)
		request.Header.Set("X-Forwarded-For", fmt.Sprintf("198.51.100.%d", index))

		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)

		want := http.StatusOK
		if index == 2 {
			want = http.StatusNotFound
		} else if index >= 3 {
			want = http.StatusTooManyRequests
		}

		if response.Code != want {
			t.Fatalf("%s: got %d, want %d", target, response.Code, want)
		}

		if index >= 1 && storage.opens.Load() != min(int64(index), 2) {
			t.Fatalf("unexpected storage access at request %d: %d opens", index, storage.opens.Load())
		}
	}
}

func TestPrivateFileServerAuthorizesBeforeCache(t *testing.T) {
	stack, err := kit.DefaultMiddleware()
	if err != nil {
		t.Fatal(err)
	}

	var verified atomic.Int64
	stack = append(stack,
		kit.Authenticate(func(ctx context.Context, request *http.Request) (kit.Principal, error) {
			verified.Add(1)
			if _, ok := ctx.Deadline(); !ok {
				t.Error("file authentication did not receive request deadline")
			}

			if request.Header.Get("Authorization") != "Bearer allowed" {
				return kit.Principal{}, kit.ErrUnauthenticated
			}

			return kit.Principal{
				ID:    "42",
				Roles: []string{"reader"},
			}, nil
		}),
		kit.RequireAnyRole("reader"),
	)
	storage := &observedFS{
		FS: fstest.MapFS{
			"private.txt": &fstest.MapFile{Data: []byte("private")},
		},
	}
	handler, err := NewHandlerWithMiddleware(storage, stack)
	if err != nil {
		t.Fatal(err)
	}

	for _, credential := range []string{
		"Bearer allowed",
		"Bearer allowed",
		"Bearer rejected",
	} {
		request := httptest.NewRequest(http.MethodGet, "/private.txt", nil)
		request.Header.Set("Authorization", credential)

		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)

		want := http.StatusOK
		if credential != "Bearer allowed" {
			want = http.StatusUnauthorized
		}

		if response.Code != want {
			t.Fatalf("got %d, want %d", response.Code, want)
		}
	}

	if verified.Load() != 3 || storage.opens.Load() != 1 {
		t.Fatalf("verified=%d storage opens=%d", verified.Load(), storage.opens.Load())
	}
}
