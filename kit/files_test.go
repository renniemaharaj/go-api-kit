package kit_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/renniemaharaj/go-api-kit/kit"
)

func TestStaticFS(t *testing.T) {
	fsys := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("home")},
	}
	handler := kit.HandlerFunc(func(h kit.Handle) {
		h.StaticFS(fsys, "index.html")
	})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusOK || response.Body.String() != "home" {
		t.Fatalf("got status %d and body %q", response.Code, response.Body.String())
	}
}

func TestStaticFSRejectsTraversal(t *testing.T) {
	handler := kit.HandlerFunc(func(h kit.Handle) {
		h.StaticFS(fstest.MapFS{}, "../secret.txt")
	})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("got status %d and body %q", response.Code, response.Body.String())
	}
}
