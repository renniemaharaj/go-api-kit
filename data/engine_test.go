package data_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/renniemaharaj/go-api-kit/data"
	"github.com/renniemaharaj/go-api-kit/kit"
)

func TestEngineMapsRoutesAndAppliesMiddlewareInOrder(t *testing.T) {
	var steps []string
	record := func(name string) kit.Middleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				steps = append(steps, name)
				next.ServeHTTP(w, r)
			})
		}
	}

	engine, err := data.NewEngine(kit.New().Use(record("global")),
		data.Definition{
			Name:       "users",
			Path:       "/users",
			Middleware: []kit.Middleware{record("resource")},
			Routes: []data.Route{
				{
					Method:     http.MethodGet,
					Path:       "/{id}",
					Middleware: []kit.Middleware{record("route")},
					Handler: func(h kit.Handle) {
						steps = append(steps, "get")
						h.Text(http.StatusOK, h.Param("id"))
					},
				},
				{
					Method: http.MethodPut,
					Path:   "/{id}",
					Handler: func(h kit.Handle) {
						steps = append(steps, "save")
						h.NoContent()
					},
				},
			},
		},
		data.Definition{
			Name: "health",
			Path: "/health",
			Routes: []data.Route{
				{
					Method: "get",
					Handler: func(h kit.Handle) {
						steps = append(steps, "health")
						h.NoContent()
					},
				},
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		method string
		path   string
		status int
		body   string
		steps  []string
	}{
		{
			method: http.MethodGet,
			path:   "/users/42",
			status: http.StatusOK,
			body:   "42",
			steps:  []string{"global", "resource", "route", "get"},
		},
		{
			method: http.MethodPut,
			path:   "/users/42",
			status: http.StatusNoContent,
			steps:  []string{"global", "resource", "save"},
		},
		{
			method: http.MethodGet,
			path:   "/health",
			status: http.StatusNoContent,
			steps:  []string{"global", "health"},
		},
	} {
		steps = nil
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, httptest.NewRequest(test.method, test.path, nil))
		if response.Code != test.status || response.Body.String() != test.body {
			t.Fatalf("%s %s: got %d %q", test.method, test.path, response.Code, response.Body.String())
		}

		if !reflect.DeepEqual(steps, test.steps) {
			t.Fatalf("%s %s: middleware order %v, want %v", test.method, test.path, steps, test.steps)
		}
	}
}

func TestEngineSubjectPolicyUsesMatchedParameters(t *testing.T) {
	authenticate := kit.Authenticate(func(context.Context, *http.Request) (kit.Principal, error) {
		return kit.Principal{ID: "42"}, nil
	})
	engine, err := data.NewEngine(kit.New().Use(authenticate), data.Definition{
		Name:       "users",
		Path:       "/users",
		Middleware: []kit.Middleware{kit.RequireSubject("id")},
		Routes: []data.Route{
			{
				Method: http.MethodGet,
				Path:   "/{id}",
				Handler: func(h kit.Handle) {
					h.Text(http.StatusOK, h.Param("id"))
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		id     string
		status int
	}{
		{
			id:     "42",
			status: http.StatusOK,
		},
		{
			id:     "57",
			status: http.StatusForbidden,
		},
	} {
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/users/"+test.id, nil))
		if response.Code != test.status {
			t.Fatalf("user %s: got %d, want %d", test.id, response.Code, test.status)
		}
	}
}

func TestEngineRejectsInvalidDefinitionsBeforeChangingRouter(t *testing.T) {
	valid := data.Definition{
		Name: "valid",
		Path: "/valid",
		Routes: []data.Route{
			{
				Method: http.MethodGet,
				Path:   "/{id}",
				Handler: func(h kit.Handle) {
					h.NoContent()
				},
			},
		},
	}

	for _, test := range []struct {
		name   string
		change func(*data.Definition)
		want   string
	}{
		{
			name: "missing name",
			change: func(definition *data.Definition) {
				definition.Name = ""
			},
			want: "name is required",
		},
		{
			name: "relative prefix",
			change: func(definition *data.Definition) {
				definition.Path = "users"
			},
			want: "resource path",
		},
		{
			name: "relative route",
			change: func(definition *data.Definition) {
				definition.Routes[0].Path = "users"
			},
			want: "route path",
		},
		{
			name: "missing handler",
			change: func(definition *data.Definition) {
				definition.Routes[0].Handler = nil
			},
			want: "handler is required",
		},
		{
			name: "nil resource middleware",
			change: func(definition *data.Definition) {
				definition.Middleware = []kit.Middleware{nil}
			},
			want: "middleware cannot be nil",
		},
		{
			name: "nil route middleware",
			change: func(definition *data.Definition) {
				definition.Routes[0].Middleware = []kit.Middleware{nil}
			},
			want: "middleware cannot be nil",
		},
		{
			name: "missing error rule target",
			change: func(definition *data.Definition) {
				definition.Errors.Rules = []kit.ErrorRule{
					{Status: http.StatusNotFound},
				}
			},
			want: "error rule requires an error",
		},
		{
			name: "invalid error response status",
			change: func(definition *data.Definition) {
				definition.Errors.Rules = []kit.ErrorRule{
					{
						Error:  data.ErrNotFound,
						Status: http.StatusOK,
					},
				}
			},
			want: "error rule status",
		},
		{
			name: "missing method",
			change: func(definition *data.Definition) {
				definition.Routes[0].Method = ""
			},
			want: "invalid route",
		},
		{
			name: "unsupported method",
			change: func(definition *data.Definition) {
				definition.Routes[0].Method = "INVALID"
			},
			want: "invalid route",
		},
		{
			name: "malformed pattern",
			change: func(definition *data.Definition) {
				definition.Routes[0].Path = "/{id"
			},
			want: "invalid route",
		},
		{
			name: "invalid regex",
			change: func(definition *data.Definition) {
				definition.Routes[0].Path = "/{id:[}"
			},
			want: "invalid route",
		},
		{
			name: "duplicate path",
			change: func(definition *data.Definition) {
				definition.Path = "/valid"
			},
			want: "conflicting route",
		},
		{
			name: "duplicate path with renamed parameter",
			change: func(definition *data.Definition) {
				definition.Path = "/valid"
				definition.Routes[0].Path = "/{name}"
			},
			want: "conflicting route",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			invalid := valid
			invalid.Path = "/invalid"
			invalid.Routes = append([]data.Route(nil), valid.Routes...)
			test.change(&invalid)

			router := kit.New()
			engine, err := data.NewEngine(router, valid, invalid)
			if engine != nil || err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("got engine %v, error %v; want %q", engine, err, test.want)
			}

			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/valid/42", nil))
			if response.Code != http.StatusNotFound {
				t.Fatal("invalid definitions partially changed the router")
			}
		})
	}

	if _, err := data.NewEngine(nil, valid); err == nil {
		t.Fatal("nil router was accepted")
	}
}
