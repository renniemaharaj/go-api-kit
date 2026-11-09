package v8

import (
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRunReturnsRootOpenError(t *testing.T) {
	err := Run(filepath.Join(t.TempDir(), "missing"), "localhost", "0")
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

	host, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	if err := Run(t.TempDir(), host, port); err == nil {
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

	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	handler := http.FileServerFS(rootFS{root})

	for _, test := range []struct {
		path string
		body string
	}{
		{path: "/", body: index},
		{path: "/welcome.svg", body: "<svg>Welcome</svg>"},
		{path: "/assets/app.js", body: "app"},
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, test.path, nil))
		if response.Code != http.StatusOK || response.Body.String() != test.body {
			t.Errorf("GET %s: got status %d and body %q", test.path, response.Code, response.Body.String())
		}
	}
}

func TestDefaultPageServesExternalStylesheetAndIcon(t *testing.T) {
	root, err := os.OpenRoot(filepath.Join("..", "dist"))
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	handler := http.FileServerFS(rootFS{root})

	page := httptest.NewRecorder()
	handler.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/", nil))
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), `id="copy-command"`) || !strings.Contains(page.Body.String(), `href="/assets/styles.css"`) {
		t.Fatalf("default page is missing interactive HTML or stylesheet link: status %d, body %q", page.Code, page.Body.String())
	}

	for _, asset := range []struct {
		path        string
		contentType string
	}{
		{path: "/assets/styles.css", contentType: "text/css"},
		{path: "/assets/v8.png", contentType: "image/png"},
		{path: "/assets/welcome.svg", contentType: "image/svg+xml"},
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

	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	handler := http.FileServerFS(rootFS{root})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/public.txt", nil))
	if response.Code == http.StatusOK && response.Body.String() == "secret" {
		t.Fatalf("served file outside frontend directory: status %d, body %q", response.Code, response.Body.String())
	}
}
