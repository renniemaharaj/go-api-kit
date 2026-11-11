# Request middleware

Middleware lives in `kit` and is installed automatically by `kit.New()` and `v8`.
Applications configure the default stack through `.env` or process variables.

```go
return v8.Run(func(router *kit.Router) {
	router.With(kit.RequireSubject("id")).Get("/users/{id}", showUser)
})
```

The route declares its access policy. Kit handles construction of the shared
middleware and runs subject checks after route parameters have been matched.

## Environment settings

Start with [.env.example](../.env.example):

```dotenv
REQUEST_TIMEOUT=15s

RATE_LIMIT_REQUESTS=20
RATE_LIMIT_INTERVAL=1s
RATE_LIMIT_BURST=40
RATE_LIMIT_MAX_CLIENTS=4096
RATE_LIMIT_IDLE_TTL=5m

CORS_ALLOWED_ORIGINS=http://localhost:3000
CORS_MAX_AGE=10m

JWT_SECRET=
JWT_ISSUER=go-api-kit-example
JWT_AUDIENCE=users-api
JWT_CONFIG_VERSION=v1
```

A missing `.env` is allowed. Process variables override file values. Settings are
loaded at construction, not during requests. The loader accepts literal quoted
or unquoted values, comments, and `export` assignments. It does not expand
variables, execute shell commands, or modify the process environment.

| Setting | Default or behavior |
| --- | --- |
| `SERVER_ADDRESS` | `localhost:9100` |
| `FRONTEND_DIR` | Empty serves APIs and health; set a directory to serve files |
| `SERVER_READ_HEADER_TIMEOUT` | `5s` |
| `SERVER_READ_TIMEOUT` | Request timeout plus five seconds |
| `SERVER_WRITE_TIMEOUT` | Request timeout plus five seconds |
| `SERVER_IDLE_TIMEOUT` | `1m` |
| `FILE_CACHE_TTL` | `1m`; `0s` disables the file-content cache |
| `FILE_CACHE_MAX_ENTRIES` | `64` |
| `FILE_CACHE_MAX_BYTES` | `1048576` |
| `REQUEST_TIMEOUT` | `15s`, must be positive |
| `RATE_LIMIT_REQUESTS` | `60` |
| `RATE_LIMIT_INTERVAL` | `1s` |
| `RATE_LIMIT_BURST` | `120` |
| `RATE_LIMIT_MAX_CLIENTS` | `4096` |
| `RATE_LIMIT_IDLE_TTL` | Automatic: at least five minutes and a full refill |
| `CORS_ALLOWED_ORIGINS` | Empty disables CORS; comma-separated origins enable it |
| `CORS_ALLOWED_METHODS` | GET, HEAD, POST, PUT, PATCH, DELETE, OPTIONS |
| `CORS_ALLOWED_HEADERS` | Authorization, Content-Type |
| `CORS_EXPOSED_HEADERS` | Retry-After |
| `CORS_ALLOW_CREDENTIALS` | `false` |
| `CORS_MAX_AGE` | `10m` |
| `JWT_ENABLED` | Enabled when a signing secret or public key is configured |
| `JWT_ALGORITHM` | `HS256`; `RS256` is also supported |
| `JWT_SECRET` | HMAC signing secret, at least 32 bytes |
| `JWT_PUBLIC_KEY` | RSA public key in PEM format for RS256; quoted `\n` escapes are accepted |
| `JWT_ISSUER` | `go-api-kit-example` |
| `JWT_AUDIENCE` | `users-api` |
| `JWT_CONFIG_VERSION` | `v1` |
| `JWT_LEEWAY` | `0s` |
| `JWT_BIND_IP` | `false` |
| `JWT_REQUIRED_ROLES` | Empty allows any verified identity; otherwise a comma-separated list |

Set `JWT_ENABLED=true` to require authentication even before a key has been
provided; startup will then fail until valid credentials are configured.
`JWT_ENABLED=false` explicitly disables shared JWT verification. Route access
checks still apply. Requiring roles without JWT authentication is a startup error.

## Default request order

1. A shared IP rate limiter.
2. A request deadline.
3. CORS, when allowed origins are configured.
4. JWT authentication, when enabled.
5. Required JWT roles, when configured.
6. Group middleware, matched-route middleware, and the handler.

CORS preflights can complete without authentication. Rate limiting still counts
them, along with denied credentials, missing routes, cache hits, and cache misses.
Group and route middleware run after matching parameters, so
`kit.RequireSubject("id")` can compare the requested ID to the verified subject.
It does not grant administrators an implicit exemption.

## Override the stack

Pass middleware to `kit.New` to replace its defaults. `Use` appends middleware to
the selected stack:

```go
router := kit.New(customAuthentication, customLogging)
router.Use(additionalMiddleware)
```

For v8 application wiring with an explicit stack:

```go
handler, err := v8.NewHandlerWithMiddleware(fsys,
	[]kit.Middleware{customAuthentication, customLogging},
	func(router *kit.Router) {
		router.Get("/hello", hello)
	},
)
if err != nil {
	return err
}
```

A nil replacement stack selects defaults. A non-nil empty slice selects no shared
middleware. Group and route policies remain active. To retain defaults and add middleware, call `router.Use(extra)` at the
start of setup, before registering routes. `v8` returns construction errors; `kit.New()` panics if its
automatic defaults cannot be constructed.

Advanced constructors remain available in `kit`: `NewRateLimiter`, `NewJWT`,
`NewCORS`, `Timeout`, `Authenticate`, and `Authorize`. They accept application functions
and explicit settings. The beginner path uses environment
settings and needs none of these initialization blocks.

## Rate limiting and identity

One limiter owns a token bucket per network peer IP. Changing resource IDs,
routes, methods, query strings, or source ports does not reset it. Excess requests
return HTTP 429 with `Retry-After`; limiting never waits for a token.

Full client tables reject new clients until idle entries expire. The configured
idle timeout must allow a full refill. Limits are local to the application
process; multiple instances need shared limits at the ingress or another store.

Peer addresses come from `Request.RemoteAddr`. `X-Forwarded-For` and `X-Real-IP`
do not change client identity. Behind a proxy, configure a trusted integration
before substituting a verified client address.

## Deadlines and JWT verification

The deadline uses `http.TimeoutHandler`: responses are buffered, expiration
returns HTTP 503, and late writes return `http.ErrHandlerTimeout`. Streaming and
connection hijacking are not supported by the default timeout middleware.

Pass `h.Context()` into services and storage operations. Existing context values
and earlier deadlines are preserved. A callback that ignores cancellation cannot
be forcibly stopped. After committed writes, invalidate cached reads with
`context.WithoutCancel(ctx)`; see the [caching guide](resources.md).

HS256 requires a secret of at least 32 bytes. RS256 requires an RSA public key of
at least 2048 bits. The algorithm and key type are pinned by configuration;
tokens cannot choose a different verification method or key source.

Tokens require a signature, the configured issuer and audience, an expiration,
a nonempty subject, and the expected `config_version`. Expired or not-yet-valid
tokens and future `iat` values are rejected, subject to configured leeway.
Malformed or duplicate authorization headers and unsigned tokens are rejected.

`kit.JWTClaims` embeds `jwt.RegisteredClaims` and adds `user_id`, `roles`,
`config_version`, and optional `ip`. When present, `user_id` must equal `sub`.
Roles are signed claims. `JWT_BIND_IP=true` requires the signed IP to match the
observed peer; forwarded headers cannot satisfy that check.

Token issuance and revocation belong to the application's identity service.
The runnable example's `-issue-token` command is a local HS256 development helper.

## CORS

Origins must be exact HTTP or HTTPS origins without paths. Wildcards are allowed
only without credentials. Unapproved origins, methods, or requested headers
return HTTP 403. Preflights return HTTP 204, and actual requests still pass through
authentication. `Vary` headers preserve browser cache separation.
