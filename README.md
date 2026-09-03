# go-handler-kit

`go-handler-kit` is a small, ergonomic HTTP handler library for Go. It uses
[chi](https://github.com/go-chi/chi) internally for routing while keeping its
public API compatible with `net/http`.

## Installation

```bash
go get github.com/renniemaharaj/go-handler-kit
```

## Quick start

```go
package main

import (
	"net/http"

	"github.com/renniemaharaj/go-handler-kit/kit"
)

func main() {
	k := kit.New()

	k.Get("/", func(h kit.Handle) {
		h.Text(http.StatusOK, "Hello World")
	})

	k.Get("/users/{id}", func(h kit.Handle) {
		_ = h.JSON(http.StatusOK, map[string]string{"id": h.Param("id")})
	})

	http.ListenAndServe(":8000", k)
}
```

`*kit.Router` implements `http.Handler`. Middleware uses the standard
`func(http.Handler) http.Handler` shape:

```go
k.Use(loggingMiddleware)

k.Route("/api", func(api *kit.Router) {
	api.Route("/users", func(users *kit.Router) {
		users.Get("/", listUsers)
		users.Post("/", createUser)
		users.Get("/{id}", getUser)
	})
})
```

JSON request bodies use terminology from `encoding/json`:

```go
func createUser(h kit.Handle) {
	var input CreateUser
	if err := h.UnmarshalJSON(&input); err != nil {
		h.Error.BadRequest("invalid body")
		return
	}

	if err := h.JSON(http.StatusCreated, input); err != nil {
		h.Error.Internal("could not encode response")
	}
}
```

Common response operations stay directly on `Handle`: `JSON`, `Text`, `HTML`,
`Status`, `NoContent`, `Redirect`, `File`, and `DistFile`. Cohesive operations
are grouped under `h.Error` and `h.Header`. `Request()` and `Writer()` expose
the underlying standard-library values whenever direct access is clearer.

`NewHandler` remains available for use with any standard-library router:

```go
mux := http.NewServeMux()
mux.HandleFunc("GET /", kit.NewHandler(func(h kit.Handle) {
	h.DistFile("index.html")
}))
```

`DistFile` serves from `./dist` and rejects paths or symlinks that escape that
directory.
