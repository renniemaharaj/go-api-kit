package kit_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/renniemaharaj/go-api-kit/kit"
)

func TestRouterRouteMatchesNestedPathAndExposesParameters(t *testing.T) {
	k := kit.New()
	k.Route("/api", func(api *kit.Router) {
		api.Route("/users", func(users *kit.Router) {
			users.Get("/{id}", func(h kit.Handle) {
				h.Text(http.StatusOK, h.Param("id")+":"+h.Query("view"))
			})
		})
	})

	response := request(t, k, http.MethodGet, "/api/users/42?view=full", "")
	if response.Code != http.StatusOK || response.Body.String() != "42:full" {
		t.Fatalf("got status %d and body %q", response.Code, response.Body.String())
	}
}

func TestRouterUseRunsStandardMiddleware(t *testing.T) {
	k := kit.New()
	k.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Middleware", "yes")
			next.ServeHTTP(w, r)
		})
	})
	k.Get("/", func(h kit.Handle) { h.NoContent() })

	response := request(t, k, http.MethodGet, "/", "")
	if response.Code != http.StatusNoContent || response.Header().Get("X-Middleware") != "yes" {
		t.Fatalf("middleware response: %#v", response.Result())
	}
}

func TestRouterRegistersHTTPMethods(t *testing.T) {
	k := kit.New()
	handler := func(h kit.Handle) { h.Status(http.StatusAccepted) }
	k.Get("/get", handler)
	k.Post("/post", handler)
	k.Put("/put", handler)
	k.Patch("/patch", handler)
	k.Delete("/delete", handler)
	k.Head("/head", handler)
	k.Options("/options", handler)

	for _, method := range []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"} {
		t.Run(method, func(t *testing.T) {
			response := request(t, k, method, "/"+strings.ToLower(method), "")
			if response.Code != http.StatusAccepted {
				t.Fatalf("got status %d", response.Code)
			}
		})
	}

	response := request(t, k, http.MethodPost, "/get", "")
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /get got status %d, want %d", response.Code, http.StatusMethodNotAllowed)
	}
}

func TestRouterGroupAppliesMiddlewareOnlyInsideGroup(t *testing.T) {
	k := kit.New()
	k.Get("/public", func(h kit.Handle) { h.NoContent() })
	k.Group(func(private *kit.Router) {
		private.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-Group", "private")
				next.ServeHTTP(w, r)
			})
		})
		private.Get("/private", func(h kit.Handle) { h.NoContent() })
	})

	public := request(t, k, http.MethodGet, "/public", "")
	if got := public.Header().Get("X-Group"); got != "" {
		t.Fatalf("public route received group header %q", got)
	}
	private := request(t, k, http.MethodGet, "/private", "")
	if got := private.Header().Get("X-Group"); got != "private" {
		t.Fatalf("private route received group header %q", got)
	}
}

func TestRouterKeepsHandlesIsolatedAcrossConcurrentRequests(t *testing.T) {
	k := kit.New()
	k.Get("/items/{id}", func(h kit.Handle) { h.Text(http.StatusOK, h.Param("id")) })

	const count = 100
	var wg sync.WaitGroup
	errors := make(chan error, count)
	for i := range count {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id := fmt.Sprint(i)
			response := httptest.NewRecorder()
			k.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/items/"+id, nil))
			if response.Code != http.StatusOK || response.Body.String() != id {
				errors <- fmt.Errorf("request %s got %d %q", id, response.Code, response.Body.String())
			}
		}()
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		t.Error(err)
	}
}

func request(t *testing.T, handler http.Handler, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(method, target, strings.NewReader(body)))
	return response
}
