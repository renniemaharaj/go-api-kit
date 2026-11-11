# Resources, repositories, and caching

Import the public package:

```go
import "github.com/renniemaharaj/go-api-kit/data"
```

The `data` package connects an application's repository to service rules and an
optional cache. It works independently of HTTP, so the same resource can serve
handlers, background jobs, and tests.

## Resource packages

Give each resource its own Go package. Its package name supplies the context,
so files and private constructors do not need a repeated resource prefix:

```text
users/
  resource.go    Model, construction, HTTP definition, and error declarations
  repository.go  Storage implementation and newRepository
  service.go     Use cases, validation, and repository operations
  handler.go     Request handling and route metadata
```

See the complete [users package](../examples/resources/users/). Its public
`New` constructor creates the repository, its cache, the service, and the
resource together. Handlers call the service; the service uses the repository.
The cache sits below the service, so cached reads still run service logic.
Applications do not wire individual repository methods or handlers.

The built-in [health resource](../v8/) shows the same layers with
narrow interfaces instead of CRUD storage. Its handler consumes `Service.Check`,
and its service consumes `Repository.Read`. The repository retains an immutable
startup snapshot; the service turns that snapshot into uptime and status.
`resource.go` constructs and connects these implementations. Tests substitute
the interfaces to exercise storage failures, cancellation, and HTTP 503 responses.

v8 registers GET and HEAD at `/health` automatically alongside application
resources and frontend files. These routes use the shared middleware and are
reserved. GET returns JSON status and uptime in seconds; HEAD omits the body.
Responses are not cached. This checks process liveness, not database readiness.

## Describe routes beside the handlers

In [handler.go](../examples/resources/users/handler.go), declare the method,
resource-relative path, handler, and any endpoint-specific middleware:

```go
func (h handler) Routes() []data.Route {
	return []data.Route{
		{
			Method:  http.MethodGet,
			Path:    "/{id}",
			Handler: h.Get,
		},
		{
			Method:     http.MethodDelete,
			Path:       "/{id}",
			Handler:    h.Delete,
			Middleware: []kit.Middleware{kit.RequireAnyRole("editor")},
		},
	}
}
```

The handler holds the service and translates HTTP input and output:

```go
type handler struct {
	service *service
}
```

Its methods call `h.service.Get`, `Save`, or `Delete`. Validation and business
operations belong in the service. The resource describes its shared prefix and
access policies:

```go
func (r *Resource) Definition(with ...kit.Middleware) data.Definition {
	handlers := handler{service: r.service}

	return data.Definition{
		Name:       "users",
		Path:       "/users",
		Middleware: with,
		Routes:     handlers.Routes(),
	}
}
```

## Run the resource application with v8

Construct the resource and pass its definition to v8:

```go
resource, err := users.New()
if err != nil {
	return err
}

return v8.Run(resource.Definition(kit.RequireSubject("id")))
```

The [runnable example](../examples/resources/main.go) follows this setup. Server
settings and shared middleware come from `.env` or process variables. There is
no application configuration file or middleware initialization block.

```dotenv
SERVER_ADDRESS=localhost:9101
FRONTEND_DIR=
REQUEST_TIMEOUT=5s
CORS_ALLOWED_ORIGINS=http://localhost:3000
```

An empty `FRONTEND_DIR` serves only resources. Set it to `dist` to serve frontend
files on the same listener. Resource paths take priority over the file fallback,
and one shared limiter covers both kinds of requests.

Inside [`v8`](../v8/), `resource.go` connects the health handler, service, and
read-only snapshot repository. It also composes application routes and delegates
frontend requests to `kit.NewFileServerFromEnv`. `v8.Run` delegates startup to
`kit.Run`; kit handles environment defaults and the frontend root's lifetime.
Application code does not construct or load settings. Kit and the private file
engine own file access, caching headers, conditional requests, ranges, and HEAD
responses.

Use `v8.NewHandler(fsys, definitions...)` for tests or an existing HTTP server.
Pass a nil filesystem for an API-only application. For direct router control:

```go
engine, err := data.NewEngine(kit.New(),
	resource.Definition(kit.RequireSubject("id")),
)
if err != nil {
	return err
}
```

Kit installs environment-driven defaults. Pass an explicit middleware stack to
`kit.New(customMiddleware)` or `v8.NewHandlerWithMiddleware` to replace them.

`Engine` implements `http.Handler`, so use it as the server's `Handler`. Add more
resource definitions to the same constructor to register the rest of the app.
Each `Definition.Path` prefixes its routes: `/users` and `/{id}` become
`/users/{id}`. An empty route path matches the resource prefix itself; `/` adds a
trailing slash. `Router.Use` returns the same router, allowing chained setup.

Middleware executes in this order:

1. Router defaults or their replacement stack, followed by additions from `Use`.
2. Shared middleware from the resource definition.
3. Middleware declared on the matched route.
4. The handler.

Resource and route middleware run after path parameters are matched, so subject
checks work without manual `router.With` calls. Policies are additive: a route
cannot remove a resource policy. Keep public and private endpoints in separate
definitions, or apply access checks only to the private routes.

The engine validates all supplied definitions before registering them. Missing
handlers, nil middleware, invalid methods or patterns, and conflicting routes
return startup errors. This validation covers the supplied definitions; routes
registered manually on the supplied router remain under that router's control.
The definitions are read at construction and are not a runtime routing registry.

## Define storage explicitly

The [repository](../examples/resources/users/repository.go) implements:

```go
type Repository[T any, ID comparable] interface {
	Get(context.Context, ID) (T, error)
	Save(context.Context, T) (T, error)
	Delete(context.Context, ID) error
}

var _ data.Repository[User, string] = (*repository)(nil)
```

`newRepository` creates the in-memory implementation in the example. Replace it
with database storage behind the same interface. The resource constructor
loads cache settings and injects the cached repository into the service:

```go
records, err := data.NewResourceFromEnv(data.ResourceConfig[User, string]{
	Name:       "users",
	Repository: storage,
	Key:        key,
})
if err != nil {
	return err
}
```

`User` is the model and `string` is its ID type. `key` extracts the ID. The
constructor retains `records` for cache invalidation and creates
`newService(records.Repository())`. This repository keeps caching and automatic
write invalidation intact.

The example `Resource.Get`, `Save`, and `Delete` delegate to the service for
non-HTTP callers too. `Invalidate` clears cached storage. The model and resource
assembly live together in `resource.go`.

Cache settings also live in the environment:

```dotenv
RESOURCE_CACHE_TTL=1m
RESOURCE_CACHE_MAX_ENTRIES=1024
USERS_CACHE_MAX_ENTRIES=256
```

`NewResourceFromEnv` defaults to a one-minute cache with 1024 entries. Resource
names form uppercase prefixes: `users` uses `USERS_CACHE_TTL` and
`USERS_CACHE_MAX_ENTRIES`. These override the corresponding `RESOURCE_CACHE_*`
settings. Set the TTL to `0s` to disable caching. The lower-level `NewResource`
constructor remains available for explicitly supplied storage policies.

## Service rules and writes

[service.go](../examples/resources/users/service.go) owns validation, context
checks, and the work performed with repository data. The handler passes input to
the service; it does not call storage or implement business rules:

```go
func (s *service) Save(ctx context.Context, user User) (User, error) {
	if err := ctx.Err(); err != nil {
		return User{}, fmt.Errorf("users: save: %w", err)
	}

	if user.ID == "" || strings.TrimSpace(user.Name) == "" {
		return User{}, fmt.Errorf("users: save: %w", errInvalidUser)
	}

	saved, err := s.repository.Save(ctx, user)
	if err != nil {
		return User{}, fmt.Errorf("users: save: %w", err)
	}

	return saved, nil
}
```

Service methods return domain errors. They remain usable by background jobs and
tests without an HTTP request. The generic `data.Resource` also offers optional
`BeforeGet`, `BeforeSave`, and `BeforeDelete` hooks for applications using it
directly. The example keeps its rules in the explicit service instead.

`Save` returns the stored value, including any ID assigned by storage. Return
success only after the transaction commits. Successful saves invalidate both
the input ID and returned ID. Successful deletes invalidate the deleted ID.
Failed writes leave existing cache entries intact. `Key` is required when
caching saves, including when supplying a `Repository` implementation.

Return `data.ErrUnsupported` from repository methods that are not supported.
Return or wrap `data.ErrNotFound` from storage when a record is absent.
Service and resource errors preserve `errors.Is` and `errors.As` behavior.
Declare HTTP responses once in the resource definition; `kit` handles mapping
and logging.

## Shared error responses

Declare known domain errors in `Definition.Errors`:

```go
Errors: kit.ErrorConfig{
	Rules: []kit.ErrorRule{
		{
			Error:   data.ErrNotFound,
			Status:  http.StatusNotFound,
			Message: "user not found",
		},
		{
			Error:   errInvalidUser,
			Status:  http.StatusBadRequest,
			Message: errInvalidUser.Error(),
		},
	},
	InternalMessage: "could not complete user operation",
},
```

The engine binds this configuration to the resource's handlers. Each handler
uses the same kit operation:

```go
user, err := h.service.Get(request.Context(), request.Param("id"))
if err != nil {
	request.Error.Handle(err)
	return
}
```

`kit` matches wrapped errors with `errors.Is` in rule order. Responses contain
only the declared message, or the standard status text when it is empty.
Unmatched errors return HTTP 500 with `InternalMessage`, defaulting to
`Internal Server Error`. Server failures are logged; their original details
stay out of responses. Passing a nil error leaves the response untouched.
The engine rejects nil rule targets and statuses outside 400-599 at startup.

For a handler outside the resource engine, call
`handler.WithErrors(config)` once and pass the returned handler to
`kit.NewHandler`. The error configuration is copied when handlers are bound.

## Explicit cache invalidation

You can remove one entry, several entries, or every entry in the current scope:

```go
if err := users.Invalidate(ctx, "42"); err != nil {
	return err
}

if err := users.Invalidate(ctx, "42", "57"); err != nil {
	return err
}

if err := users.InvalidateAll(ctx); err != nil {
	return err
}
```

Invalidation removes cached data without reading or changing storage. The next
`Get` loads fresh data. An empty ID list is a no-op. With caching disabled,
invalidation is also a no-op, unless the context is already canceled.

Call invalidation after a successful commit when another code path changes
storage. It is also how you connect custom writes to affected cached reads.
For example, if a team update changes a cached user summary, invalidate that
user explicitly. The library cannot infer dependencies between resources.

Reads already in progress may still return the data they loaded. Invalidation
prevents those reads from putting their older results back into the cache.
It does not cancel reads or provide transactional consistency with storage.

## Cache behavior and ownership

- A positive `TTL` enables caching. Zero disables it.
- The cache is local to each resource instance in each process.
- `MaxEntries` defaults to 1024 and applies across all scopes in that instance.
- Least recently used entries are evicted when the cache reaches capacity.
- Only successful reads are cached. Errors and missing records are retried.
- Successful saves and deletes invalidate affected IDs without preloading them.
- Concurrent misses may each call storage. Requests are not coalesced.
- External changes require explicit invalidation or waiting for expiration.

Treat cached values as immutable. For pointers, maps, slices, or structs
containing them, provide a deep-copy function if the values may be modified:

```go
Clone: func(user User) User {
	user.Roles = append([]string(nil), user.Roles...)
	return user
},
```

`Clone` copies values into the cache and copies them again on cache hits. On a
miss, the caller receives the storage result. Storage must return a value that
the caller can safely own, and it must not mutate it while it is being cloned.
Callbacks, hooks, scope functions, and clone functions must be concurrency-safe.

## Separating tenants or caller-dependent data

By default, IDs share a cache within the resource. If the same ID can return
different data for different tenants or callers, supply a scope:

```go
Scope: func(ctx context.Context) (string, error) {
	tenant, err := tenantFromContext(ctx)
	if err != nil {
		return "", err
	}

	return tenant.ID, nil
},
```

The cache key combines scope and ID. `Invalidate` and `InvalidateAll` affect only
the scope resolved from their context. Without a scope function,
`InvalidateAll` clears the resource's entire cache. The scope must include every
caller attribute that changes the storage result, and remain stable throughout
an operation. Storage still needs to enforce its own tenant boundaries.

Each application process owns its cache. An invalidation in one process does not
reach another process; applications with several instances need to deliver
invalidation events to each instance or use an appropriate expiration policy.

## Extending the service

Add business operations to `service.go`. They can load repository data, change
it, and save through the same validated operations:

```go
func (s *service) Rename(ctx context.Context, id, name string) (User, error) {
	user, err := s.Get(ctx, id)
	if err != nil {
		return User{}, err
	}

	user.Name = name
	return s.Save(ctx, user)
}
```

`records.Repository()` preserves caching and automatic write invalidation. It
bypasses the generic resource's hooks, so the service owns its validation and
access rules. The example constructor injects this repository once; HTTP
handlers always use the service.

## Storage callbacks

For a small adapter, `ResourceConfig` also accepts named `Load`, `Save`, and
`Delete` functions instead of `Repository`. The constructor rejects mixing these
forms. `Load` is required; omitted writes return `data.ErrUnsupported`.

```go
users, err := data.NewResource(data.ResourceConfig[User, string]{
	Name:   "users",
	Key:    userKey,
	Load:   findUserByID,
	Save:   saveUser,
	Delete: deleteUser,
})
if err != nil {
	return err
}
```

## HTTP example

The runnable example uses an explicit user resource and repository, validation,
caching, JWT authentication, subject checks, CORS, a shared IP limiter, and request
deadlines.
It accepts CORS requests from `http://localhost:3000`.

In one terminal, generate a signing secret, print a development JWT for user 42,
and start the server:

```sh
cp .env.example .env
export JWT_SECRET="$(openssl rand -hex 32)"
go run ./examples/resources -issue-token 42
SERVER_ADDRESS=localhost:9101 FRONTEND_DIR= go run ./examples/resources
```

In another terminal, set `resource_token` to the printed JWT and call the API:

```sh
resource_token='paste-the-issued-token'
curl -H "Authorization: Bearer $resource_token" http://localhost:9101/users/42
curl -X PUT http://localhost:9101/users/42 \
  -H "Authorization: Bearer $resource_token" \
  -H 'Content-Type: application/json' \
  -d '{"name":"Grace"}'
curl -H "Authorization: Bearer $resource_token" http://localhost:9101/users/42
curl -X DELETE -H "Authorization: Bearer $resource_token" http://localhost:9101/users/42
```

The token expires after 15 minutes and cannot access `/users/57`. The issuance
command is a local development helper; the HTTP API has no token issuance route.
Applications should obtain tokens from their identity service. See the
[middleware guide](middleware.md) for verification and IP-binding rules.
The data store and cache both live in memory and reset on restart.
