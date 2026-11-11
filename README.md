# go-api-kit

<p align="center">
  <img src="dist/assets/v8.png" alt="Go API Kit app icon" width="96">
</p>

Build resource APIs with **handler -> service -> cached repository** wiring.
`kit` supplies the default request middleware and server runtime. `v8` composes
application resources and delegates startup to kit. Environment defaults are
loaded by the engines; application code passes no settings or configuration.

## Start the bundled server

```sh
cp .env.example .env
go run .
```

The entrypoint stays small:

```go
func main() {
	if err := v8.Run(); err != nil {
		log.Fatal(err)
	}
}
```

Edit `.env` to change the address, frontend directory, request limits, timeouts,
caching, CORS, or JWT settings. Process variables override file values.

Every v8 application includes `GET /health` and `HEAD /health`. A successful GET
returns HTTP 200 with `{"status":"ok","uptime_seconds":0}`; uptime increases
from handler construction. HEAD returns the same status without a body. Health
responses use `Cache-Control: no-store`.

The built-in [health resource](v8/) demonstrates a handler calling a
service interface, with the service reading a startup snapshot through a
read-only repository interface. This is process liveness; external dependencies
are not checked. Service failures return HTTP 503 with status `unavailable`.
Health uses the shared middleware, including rate limits and JWT authentication
when enabled. Probes must supply credentials when authentication is configured.
The GET and HEAD routes at `/health` are reserved; conflicting definitions fail
at startup.

```dotenv
SERVER_ADDRESS=localhost:9100
FRONTEND_DIR=dist
REQUEST_TIMEOUT=15s
RATE_LIMIT_REQUESTS=20
RATE_LIMIT_INTERVAL=1s
RATE_LIMIT_BURST=40
CORS_ALLOWED_ORIGINS=http://localhost:3000
```

## Add a resource

Each resource package contains:

```text
users/
  resource.go    Model, construction, route and error declarations
  repository.go  Storage operations
  service.go     Business rules and repository use
  handler.go     HTTP input, output, and endpoint declarations
```

Construct the resource and start it:

```go
resource, err := users.New(users.User{
	ID:   "42",
	Name: "Ada",
})
if err != nil {
	return err
}

return v8.Run(resource.Definition(kit.RequireSubject("id")))
```

`v8` registers the declared routes through `data.NewEngine`. It also serves
frontend files when `FRONTEND_DIR` is set. Leave that variable empty for an
API-only server. Resource routes take priority over the frontend file fallback.

The [runnable example](examples/resources/main.go) shows the complete flow.
The [resource guide](docs/resources.md) covers services, validation, errors,
caching, and resource-specific access rules.

## Default middleware

`kit.New()` and `v8` install a shared IP limiter and request deadline. CORS is
activated by `CORS_ALLOWED_ORIGINS`. JWT verification is activated by a configured
`JWT_SECRET` or `JWT_PUBLIC_KEY`; `JWT_ENABLED` can explicitly enable or disable it.
Optional required roles come from `JWT_REQUIRED_ROLES`.

Middleware implementations live in `kit`. Applications do not need to construct
JWT, CORS, timeout, or rate-limit configuration objects to use the defaults.

```go
router := kit.New()                 // Environment-driven defaults.
router.Use(logging)                 // Append middleware.
custom := kit.New(customMiddleware) // Replace the default stack.
```

`v8.NewHandler(fsys, definitions...)` returns the default application handler for
tests or an existing HTTP server. Pass nil for an API without frontend files.
`v8.NewHandlerWithMiddleware(fsys, stack, definitions...)` supplies an explicit
replacement stack. Invalid environment settings return startup errors from `v8`;
`kit.New()` panics on invalid defaults because its constructor returns one value.
See the [middleware guide](docs/middleware.md) for environment variables and
advanced overrides.

## Packages

```text
go-api-kit/
  kit/        HTTP runtime, routing, handlers, files, errors, and middleware
  v8/         Application resources and startup composition
  data/       Resource definitions, repositories, caching, and HTTP wiring
  internal/   Private storage, cache, filesystem, and environment implementations
  examples/   Runnable applications
  docs/       Resource and middleware guides
```

The `v8` package uses the same resource layout for its built-in health endpoint.
`resource.go` connects the handler, service, and repository; `handler.go` handles
HTTP, `service.go` computes health status, and `repository.go` keeps the startup
snapshot. Kit owns server defaults and file serving; the private file engine
owns file access and content caching.

Install and import the public packages directly:

```sh
go get github.com/renniemaharaj/go-api-kit
```

```go
import (
	"github.com/renniemaharaj/go-api-kit/data"
	"github.com/renniemaharaj/go-api-kit/kit"
	"github.com/renniemaharaj/go-api-kit/v8"
)
```

`*kit.Router` and `*data.Engine` implement `http.Handler`. Handler conveniences
include `JSON`, `UnmarshalJSON`, `Text`, `HTML`, `NoContent`, `File`, and `StaticFS`.
Use `h.Error.Handle(err)` for resource-declared error responses and logging.
`Request()` and `Writer()` expose the standard-library HTTP values.
