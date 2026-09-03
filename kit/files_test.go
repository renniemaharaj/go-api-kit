package kit_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/renniemaharaj/go-handler-kit/kit"
)

func TestDistFile(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.Mkdir("dist", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join("dist", "index.html"), []byte("home"), 0o644); err != nil {
		t.Fatal(err)
	}

	handler := kit.NewHandler(func(h kit.Handle) { h.DistFile("") })
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusOK || response.Body.String() != "home" {
		t.Fatalf("got status %d and body %q", response.Code, response.Body.String())
	}
}

func TestDistFileRejectsTraversal(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.Mkdir("dist", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("secret.txt", []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}

	handler := kit.NewHandler(func(h kit.Handle) { h.DistFile("../secret.txt") })
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusBadRequest || response.Body.String() == "secret" {
		t.Fatalf("got status %d and body %q", response.Code, response.Body.String())
	}
}

func TestDistFileRejectsEscapingSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation commonly requires extra privileges on Windows")
	}
	t.Chdir(t.TempDir())
	if err := os.Mkdir("dist", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("secret.txt", []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("..", "secret.txt"), filepath.Join("dist", "public.txt")); err != nil {
		t.Fatal(err)
	}

	handler := kit.NewHandler(func(h kit.Handle) { h.DistFile("public.txt") })
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusBadRequest || response.Body.String() == "secret" {
		t.Fatalf("got status %d and body %q", response.Code, response.Body.String())
	}
}
