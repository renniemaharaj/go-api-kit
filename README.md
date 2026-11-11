# go-api-kit

<p align="center">
  <img src="dist/assets/v8.png" alt="Go API Kit app icon" width="96">
</p>

A small Go HTTP toolkit. Register handlers directly; use storage, business logic,
and caching when your application needs them.

```sh
go get github.com/renniemaharaj/go-api-kit
```

Start an application with environment-driven middleware, optional frontend files,
and a built-in health check:

```go
func main() {
	if err := v8.Run(); err != nil {
		log.Fatal(err)
	}
}
```

Add an endpoint in the same call:

```go
if err := v8.Run(func(router *kit.Router) {
	router.Get("/hello", func(h kit.Handle) {
		h.Text(http.StatusOK, "Hello, world!")
	})
}); err != nil {
	log.Fatal(err)
}
```

See the complete [minimal example](examples/minimal/main.go). Run it with
`go run ./examples/minimal`. No repository, service interface, or route definition
is required. Use `Route`, `Group`, and `With` for optional grouping and policies:

```go
router.Route("/users", func(users *kit.Router) {
	users.With(kit.RequireSubject("id")).Get("/{id}", showUser)
})
```

`*kit.Router` and `kit.HandlerFunc` implement `http.Handler`. Register a standard handler directly with
`router.Handle(http.MethodGet, "/legacy", legacyHandler)`. Kit handlers expose
`Request()`, `Writer()`, `Context()`, `DecodeJSON`, JSON, text, HTML, and file helpers.

## Run the bundled frontend

```sh
cp .env.example .env
go run .
```

A missing `.env` is allowed. Process variables override file values. Set
`FRONTEND_DIR=dist` to serve the bundled page, or leave it empty for an API-only
application. Explicit routes take priority over frontend files.

```dotenv
SERVER_ADDRESS=localhost:9100
FRONTEND_DIR=dist
REQUEST_TIMEOUT=15s
RATE_LIMIT_REQUESTS=20
RATE_LIMIT_INTERVAL=1s
RATE_LIMIT_BURST=40
CORS_ALLOWED_ORIGINS=http://localhost:3000
```

`GET /health` returns `{"status":"ok","uptime_seconds":0}` with uptime measured
from handler construction. `HEAD /health` returns the same status without a body.
Both use `Cache-Control: no-store`. Health checks process liveness and uses the
shared middleware, including JWT authentication when configured. GET and HEAD
at `/health` are reserved; conflicting registrations return startup errors.

## Middleware and HTTP integration

`kit.New()` and `v8` install a shared IP limiter and request deadline. CORS is
enabled by `CORS_ALLOWED_ORIGINS`; a configured JWT key enables authentication.
`JWT_REQUIRED_ROLES` adds a shared role policy. Configuration errors fail startup.
See the [middleware guide](docs/middleware.md) for all settings and overrides.

```go
router := kit.New()                  // Environment-driven defaults.
router.Use(logging)                 // Append before registering routes.
custom := kit.New(customMiddleware)  // Replace defaults.
```

`v8.NewHandler(fsys, setup...)` builds a handler for tests or an existing HTTP
server. A nil filesystem serves APIs and health only, even without setup.
`v8.NewHandlerWithMiddleware(fsys, stack, setup...)` accepts a replacement stack;
a non-nil empty slice disables defaults. Setup functions receive `*kit.Router`.
Invalid routes, duplicate patterns, nil handlers, and nil middleware panic during
manual router setup; v8 converts setup panics to errors and returns no handler.
Register all routes and middleware before serving requests.

## Optional caching and business logic

`data.NewCache(load, data.CacheConfig[T]{TTL: time.Minute})` caches a loading
function. Zero TTL disables caching. The cache supports bounded LRU eviction,
scopes, cloning mutable values, and explicit invalidation. Applications own
writes and invalidate affected keys after a successful commit.

The [advanced example](examples/resources/) combines concrete user storage,
validation, caching, JWT subject checks, and HTTP error mapping in two user files.
The [storage and caching guide](docs/resources.md) explains ownership,
cancellation-safe invalidation, and tenant separation.

| Package | Responsibility |
| --- | --- |
| `kit` | Router, HTTP helpers, middleware, server runtime, safe file serving |
| `v8` | Application startup, health, and optional frontend composition |
| `data` | Optional loading-function cache; independent of HTTP |
| `internal` | Bounded cache, file contents, and environment parsing |

See [refactor measurements and migration notes](docs/simplification.md) for the
breaking changes and the size of the simplification.
