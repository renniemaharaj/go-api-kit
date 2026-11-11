# Routes, storage, and caching

Register endpoints directly with `kit.Router`. An endpoint can call a function,
a concrete application type, or a database driver. The toolkit imposes no
handler/service/repository layout.

```go
return v8.Run(func(router *kit.Router) {
	router.Get("/hello", func(h kit.Handle) {
		h.Text(http.StatusOK, "Hello, world!")
	})
})
```

Grouping and access policies are optional:

```go
router.Route("/users", func(users *kit.Router) {
	users.With(kit.RequireSubject("id")).Get("/{id}", showUser)
})
```

Global middleware runs first. Group and endpoint middleware added with `With`
run after matching path parameters, in declaration order. A cached read still
passes through authentication and authorization on every HTTP request. Keep
business checks that must run on every read outside the cached loading function.

Use `router.Handle(method, path, httpHandler)` for a standard `http.Handler`.
`kit.HandlerFunc(fn)` implements `http.Handler` directly for other routers or servers.
`v8.Run(setup...)` adds health and optional frontend files; `kit.Run(build)`
remains available when you want to provide the entire application handler.

## Optional loading-function cache

`data` has no dependency on HTTP. Wrap any context-aware loading function:

```go
users, err := data.NewCache(findUser, data.CacheConfig[User]{
	TTL:        time.Minute,
	MaxEntries: 256,
})
if err != nil {
	return err
}

user, err := users.Get(ctx, "42")
```

The loading function has signature `func(context.Context, ID) (T, error)`. It can
load a record, calculate a report, or fetch a remote result. There are no required
write operations, key extraction functions, resource names, or interfaces.
Pass a concrete storage method directly when you have one.

Zero TTL disables caching and allocates no entry store. Positive TTL opts in;
`MaxEntries` defaults to 1024. Negative settings and a nil loader return errors.
Application data is never implicitly cached from environment settings. The
advanced example explicitly enables a one-minute cache in its constructor.

## Writes and invalidation

The application owns its storage operations. Validate input, commit the change,
then invalidate every affected key:

```go
saved, err := storage.Save(ctx, user)
if err != nil {
	return User{}, err
}

if err := users.Invalidate(context.WithoutCancel(ctx), user.ID, saved.ID); err != nil {
	return User{}, err
}

return saved, nil
```

Capture the original ID before storage runs if storage may mutate a pointer or
assign a new ID. Invalidate both the original and returned IDs. Failed writes
leave the cache intact. After a successful delete, invalidate the deleted ID.
For custom business operations, invalidate all dependent keys. For external
writes, deliver invalidation to each application process or wait for expiration.

Use `context.WithoutCancel(ctx)` after commit: the request deadline must not skip
cache invalidation for data already changed. It preserves scope values while
removing cancellation. Scope functions used here should resolve stable context
values locally. Handle scope resolution errors; do not report success with stale
cached data. The [users example](../examples/resources/users/users.go) holds its
storage lock through invalidation and demonstrates this sequence for writes.

`Invalidate(ctx, ids...)` removes keys in the current scope. An empty ID list
changes nothing. `InvalidateAll(ctx)` clears the current scope. Without a scope
function it clears this cache instance. Both honor context cancellation; with
caching disabled they perform no cache changes.

Invalidation fences loads already in flight: those loads may return their older
result to their existing caller, but cannot put that result back into the cache.
The cache does not provide storage transactions or cancel existing reads.

## Cache ownership and scopes

- Entries are local to each cache instance in each process.
- Only successful, uncanceled loads are cached; errors and missing values retry.
- TTL expires entries, and bounded LRU eviction limits retained entries.
- Concurrent misses can load independently; requests are not coalesced.
- Contexts, deadlines, and values reach the loader unchanged.
- Loading, scope, and clone callbacks must be safe for concurrent calls.

Treat cached values as immutable. Supply a deep copy for pointers, slices, maps,
or structs containing mutable fields:

```go
Clone: func(user User) User {
	user.Roles = append([]string(nil), user.Roles...)
	return user
},
```

Values are cloned when entering the cache and on cache hits. On misses, the
caller receives the storage result; storage must allow the caller to own it and
must not mutate it while cloning occurs.

When a key can produce different values for different callers or tenants, include
every relevant attribute in `Scope`:

```go
Scope: func(ctx context.Context) (string, error) {
	tenant, err := tenantFromContext(ctx)
	if err != nil {
		return "", err
	}

	return tenant.ID, nil
},
```

Scope is resolved for reads and invalidation, including when caching is disabled.
Errors fail the operation before storage is called. Keys combine scope and ID;
capacity is shared across scopes. Storage must still enforce tenant boundaries.
Keep the scope stable throughout an operation and preserve it after commit.

## HTTP errors

A handler can write errors directly with `h.Error.BadRequest`, `NotFound`, or
`Internal`, or bind reusable domain mappings once during route setup:

```go
handler, err := kit.HandlerFunc(showUser).WithErrors(kit.ErrorConfig{
	Rules: []kit.ErrorRule{
		{Error: errNotFound, Status: http.StatusNotFound, Message: "user not found"},
	},
	InternalMessage: "could not complete user operation",
})
if err != nil {
	return err
}

router.Get("/users/{id}", handler)
```

Handlers call `h.Error.Handle(err)`. Rules match wrapped errors with `errors.Is`
in order. Empty messages use standard status text. Unmatched errors return HTTP
500 with the configured fallback. Server errors are logged without exposing
private details. Nil errors leave the response untouched. Binding validates rule
targets and status codes and copies the rules.

## Advanced example

[users.go](../examples/resources/users/users.go) combines synchronized in-memory
storage, validation, and optional caching in one concrete type. Its embedded cache
supplies reads and invalidation. [handler.go](../examples/resources/users/handler.go)
registers routes directly and handles HTTP input and output. Applications can
extract business or storage layers when that provides a useful boundary.

The runnable example retains JWT verification, subject policies, CORS, the shared
IP limiter, and request deadlines. To run it:

```sh
cp .env.example .env
export JWT_SECRET="$(openssl rand -hex 32)"
go run ./examples/resources -issue-token 42
SERVER_ADDRESS=localhost:9101 FRONTEND_DIR= go run ./examples/resources
```

Set `resource_token` to the printed token in another terminal:

```sh
resource_token='paste-the-issued-token'
curl -H "Authorization: Bearer $resource_token" http://localhost:9101/users/42
curl -X PUT http://localhost:9101/users/42 \
  -H "Authorization: Bearer $resource_token" \
  -H 'Content-Type: application/json' -d '{"name":"Grace"}'
curl -X DELETE -H "Authorization: Bearer $resource_token" http://localhost:9101/users/42
```

The token expires after 15 minutes and cannot access another user's ID. Issuance
is a local development helper. The example's storage and cache reset on restart.
