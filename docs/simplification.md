# Simplification measurements and migration

The original dependency graph put `v8` and application resources above
`data.Engine`, which in turn wrapped `kit.Router`. Generic storage passed through
`Resource`, a repository interface, a cached repository, and a function adapter.
Health used handler and service interfaces to read a startup timestamp. The users
example repeated these layers across four production files.

The new application path is `v8` to `kit.Router` to the registered handler.
Optional data loading uses `data.Cache`, backed by the existing bounded cache.
File serving reuses that cache without implementing unsupported CRUD operations.

## Measurements

Counts compare the initial repository HEAD with the refactored working tree.
Production Go counts include comments and blank lines and exclude `_test.go`.
Public API counts use Go AST declarations: exported types, variables, functions,
and methods on exported receivers in `kit`, `data`, and `v8`. Struct fields,
interface members, and promoted methods are excluded from declaration counts.

| Measure | Before | After | Reduction |
| --- | ---: | ---: | ---: |
| All production Go lines, including examples | 3,526 | 2,724 | 802 (23%) |
| Toolkit and entrypoint production Go lines | 3,155 | 2,439 | 716 (23%) |
| Production Go files, including examples | 46 | 33 | 13 |
| Public API declarations | 109 | 88 | 21 (19%) |
| `data` production lines | 494 | 145 | 349 (71%) |
| `v8` production lines | 226 | 107 | 119 (53%) |
| Advanced users package lines | 294 | 186 | 108 (37%) |
| Advanced users package files | 4 | 2 | 2 |

The complete minimal example is 20 lines in one file, including imports and
startup error handling. Its endpoint is a three-line `router.Get` call. Compared
with the former 294-line users package, this is substantially less setup for a
basic endpoint; the minimal example deliberately has fewer responsibilities than
the CRUD example. The advanced users package retains those CRUD capabilities in
186 lines and includes validation and explicit caching.

## Removed interfaces and wrappers

Four repository-defined interfaces are gone: `data.Repository`,
`internal/repository.Store`, `v8.Service`, and `v8.Repository`. Application loading
functions provide substitution directly. Standard `http.Handler`, `fs.FS`, and
file capability interfaces remain useful interoperability boundaries.

Twenty-three named structs were removed or replaced, and four were introduced,
leaving nineteen fewer named structs overall:

- `data`: removed `Engine`, `Definition`, `Route`, `resourceRoute`, `Resource`,
  `ResourceConfig`, and `CachePolicy`; introduced `Cache` and `CacheConfig`.
- `internal/repository`: removed `Cached`, `Functions`, and its `CacheConfig`.
- `v8`: removed `handler`, `service`, `repository`, `Snapshot`, and `Status`.
  Health now closes over its startup time.
- Users example: replaced `Resource`, `handler`, `service`, and `repository` with
  one concrete `Users` type. Its embedded cache provides reads and invalidation.
- File serving: removed `fileRepository`, reused the internal file configuration
  through the public `FileCacheConfig` alias, and removed the filesystem `Root`
  wrapper in favor of `os.Root.FS()`.
- HTTP helpers: removed `HeaderAPI` and the `NewHandler` adapter. `HandlerFunc`
  implements `http.Handler` directly; `DecodeJSON` replaces the per-request decoder
  closure. Standard request and response headers remain directly accessible.
- Router: added one private route registry to retain startup validation.

The `internal/repository` and `internal/filesystem` packages were deleted.
`data` is now independent of HTTP. `v8` composes routes and built-ins directly.
No external dependencies were added.

## Deliberate breaking changes

- `v8.Run`, `NewHandler`, and `NewHandlerWithMiddleware` accept
  `func(*kit.Router)` setup callbacks in place of resource definitions.
- `data.Engine`, route definitions, generic CRUD resources, repository contracts,
  service hooks, and their constructors were removed. Register routes directly;
  place application rules in ordinary functions or concrete methods.
- `data.Cache` replaces generic resource caching. It wraps a loader, and exposes
  `Get`, `Invalidate`, and `InvalidateAll`. It has no required writes or resource
  names. Domain errors belong to the application; the toolkit no longer declares
  `data.ErrNotFound` or `data.ErrUnsupported`.
- Application caching requires an explicit positive TTL. `RESOURCE_CACHE_*` and
  resource-specific cache environment variables are no longer read. File cache
  environment settings remain supported.
- Writes belong to the application. After commit, invalidate affected IDs,
  including original and assigned IDs, using `context.WithoutCancel(ctx)`.
  Failed writes must leave the cache intact. See the [caching guide](resources.md).
- Bind HTTP error mappings with `HandlerFunc.WithErrors` during setup instead of
  declaring them in a resource definition.
- Replace `kit.NewHandler(fn)` with `kit.HandlerFunc(fn)`, which implements
  `http.Handler` itself. Use `h.DecodeJSON` instead of `h.UnmarshalJSON`.
  Replace `h.Header` with `h.Request().Header` or `h.Writer().Header()`.
- Health's former service and snapshot types were removed. Its HTTP payload,
  headers, uptime, and shared security policies remain.
- Invalid and conflicting manual router registration now panics during setup.
  v8 converts setup panics to startup errors and exposes no partial handler.
  Validation also covers routes registered directly, including nested groups.
- `v8.NewHandler(nil)` and `v8.Run()` now support health-only applications without
  requiring a filesystem or application routes.

## Retained behavior and validation

Environment-driven server configuration and defaults remain. Request order stays
rate limiting, deadline, optional CORS, optional JWT, shared roles, and matched
route policies. Authentication and authorization still run before cached reads.
Direct handlers, grouping, request parameters, custom middleware, and standard
HTTP interoperability remain available through the router.

File serving retains confined `os.Root` access, bounded content caching, oversized
file streaming, independent request readers, conditional responses, ranges,
ETags, and HEAD semantics. API and health routes take priority over the frontend.
The limiter remains shared across files, APIs, health, and missing routes.

Caching retains context propagation, scoped keys and invalidation, deep copying,
bounded LRU eviction, expiry, concurrent access, and generation fencing against
stale in-flight loads. It remains process-local and does not coalesce misses or
provide transactional reads. These limits are documented rather than hidden
behind repository abstractions.

Tests for removed layers were replaced with behavioral regressions covering
direct routing, middleware order, setup errors, health-only startup, cancellation,
cache ownership, scoped invalidation, stale-load fencing, and concurrent use.
Existing JWT, CORS, rate limiting, timeout, safe file serving, HTTP response, and
end-to-end user tests remain.

Validation commands: `gofmt`, `go vet ./...`, `go test ./...`,
`go test -race ./...`, and `go build ./...`.
