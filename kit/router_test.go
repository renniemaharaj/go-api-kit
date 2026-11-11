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
	router := kit.New()
	router.Route("/api", func(api *kit.Router) {
		api.Route("/users", func(users *kit.Router) {
			users.Get("/{id}", func(h kit.Handle) {
				h.Text(http.StatusOK, h.Param("id")+":"+h.Query("view"))
			})
		})
	})

	response := request(t, router, http.MethodGet, "/api/users/42?view=full", "")
	if response.Code != http.StatusOK || response.Body.String() != "42:full" {
		t.Fatalf("got status %d and body %q", response.Code, response.Body.String())
	}
}

func TestRouterUseRunsStandardMiddleware(t *testing.T) {
	router := kit.New().Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Middleware", "yes")
			next.ServeHTTP(w, r)
		})
	})
	router.Get("/", func(h kit.Handle) {
		h.NoContent()
	})

	response := request(t, router, http.MethodGet, "/", "")
	if response.Code != http.StatusNoContent || response.Header().Get("X-Middleware") != "yes" {
		t.Fatalf("middleware response: %#v", response.Result())
	}
}

func TestRouterRegistersHTTPMethods(t *testing.T) {
	router := kit.New()
	handler := func(h kit.Handle) {
		h.Status(http.StatusAccepted)
	}
	router.Get("/get", handler)
	router.Post("/post", handler)
	router.Put("/put", handler)
	router.Patch("/patch", handler)
	router.Delete("/delete", handler)
	router.Head("/head", handler)
	router.Options("/options", handler)

	for _, method := range []string{
		"GET",
		"POST",
		"PUT",
		"PATCH",
		"DELETE",
		"HEAD",
		"OPTIONS",
	} {
		t.Run(method, func(t *testing.T) {
			response := request(t, router, method, "/"+strings.ToLower(method), "")
			if response.Code != http.StatusAccepted {
				t.Fatalf("got status %d", response.Code)
			}
		})
	}

	response := request(t, router, http.MethodPost, "/get", "")
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /get got status %d, want %d", response.Code, http.StatusMethodNotAllowed)
	}
}

func TestRouterGroupAppliesMiddlewareOnlyInsideGroup(t *testing.T) {
	router := kit.New()
	router.Get("/public", func(h kit.Handle) {
		h.NoContent()
	})
	router.Group(func(private *kit.Router) {
		private.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-Group", "private")
				next.ServeHTTP(w, r)
			})
		})
		private.Get("/private", func(h kit.Handle) {
			h.NoContent()
		})
	})

	public := request(t, router, http.MethodGet, "/public", "")
	if got := public.Header().Get("X-Group"); got != "" {
		t.Fatalf("public route received group header %q", got)
	}

	private := request(t, router, http.MethodGet, "/private", "")
	if got := private.Header().Get("X-Group"); got != "private" {
		t.Fatalf("private route received group header %q", got)
	}
}

func TestRouterKeepsHandlesIsolatedAcrossConcurrentRequests(t *testing.T) {
	router := kit.New()
	router.Get("/items/{id}", func(h kit.Handle) {
		h.Text(http.StatusOK, h.Param("id"))
	})

	const count = 100

	var requests sync.WaitGroup
	failures := make(chan error, count)
	for i := range count {
		requests.Go(func() {
			id := fmt.Sprint(i)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/items/"+id, nil))
			if response.Code != http.StatusOK || response.Body.String() != id {
				failures <- fmt.Errorf("request %s got %d %q", id, response.Code, response.Body.String())
			}
		})
	}

	requests.Wait()
	close(failures)
	for err := range failures {
		t.Error(err)
	}
}

func request(t *testing.T, handler http.Handler, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(method, target, strings.NewReader(body)))
	return response
}

func TestDirectHandlersAndNestedPoliciesRunInOrder(t *testing.T) {
	var steps []string
	record := func(name string) kit.Middleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				steps = append(steps, name)
				next.ServeHTTP(w, r)
			})
		}
	}

	router := kit.New(record("global"))
	router.Route("/users", func(users *kit.Router) {
		users.With(record("group")).With(record("route")).Handle("GET", "/{id}", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			steps = append(steps, r.PathValue("id"))
			w.WriteHeader(http.StatusNoContent)
		}))
	})

	response := request(t, router, "GET", "/users/42", "")
	if response.Code != http.StatusNoContent || strings.Join(steps, ",") != "global,group,route,42" {
		t.Fatalf("got %d, order %v", response.Code, steps)
	}
}

func TestRouterRejectsInvalidAndConflictingRegistration(t *testing.T) {
	handle := func(h kit.Handle) { h.NoContent() }
	for _, test := range []struct {
		name  string
		setup func(*kit.Router)
	}{
		{name: "nil handler", setup: func(r *kit.Router) { r.Get("/invalid", nil) }},
		{name: "nil http handler", setup: func(r *kit.Router) { r.Handle("GET", "/invalid", nil) }},
		{name: "nil middleware", setup: func(r *kit.Router) { r.With(nil) }},
		{name: "nil global middleware", setup: func(r *kit.Router) { r.Use(nil) }},
		{name: "method", setup: func(r *kit.Router) { r.Method("INVALID", "/invalid", handle) }},
		{name: "relative path", setup: func(r *kit.Router) { r.Get("invalid", handle) }},
		{name: "pattern", setup: func(r *kit.Router) { r.Get("/{id", handle) }},
		{name: "regex", setup: func(r *kit.Router) { r.Get("/{id:[}", handle) }},
		{name: "duplicate", setup: func(r *kit.Router) {
			r.Get("/users/{id}", handle)
			r.With().Get("/users/{name}", handle)
		}},
		{name: "nested duplicate", setup: func(r *kit.Router) {
			r.Get("/api/users/{id}", handle)
			r.Route("/api/", func(api *kit.Router) {
				api.Group(func(group *kit.Router) {
					group.Get("/users/{name}", handle)
				})
			})
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("invalid setup was accepted")
				}
			}()
			test.setup(kit.New([]kit.Middleware{}...))
		})
	}
}
