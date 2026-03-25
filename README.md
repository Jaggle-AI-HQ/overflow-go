# overflow-go

[![CI](https://github.com/Jaggle-AI-HQ/overflow-go/actions/workflows/ci.yml/badge.svg)](https://github.com/Jaggle-AI-HQ/overflow-go/actions/workflows/ci.yml)
[![Latest Release](https://img.shields.io/github/v/release/Jaggle-AI-HQ/overflow-go)](https://github.com/Jaggle-AI-HQ/overflow-go/releases/latest)

Go SDK for [Overflow](https://overflow.jaggle.ai) error tracking.

## Installation

```bash
go get github.com/Jaggle-AI-HQ/overflow-go
```

## Quick Start

```go
package main

import (
    "log"
    "time"

    overflow "github.com/Jaggle-AI-HQ/overflow-go"
)

func main() {
    err := overflow.Init(overflow.ClientOptions{
        DSN:         "https://<public-key>@your-host.com/api/ingest",
        Environment: "production",
        Release:     "myapp@1.0.0",
    })
    if err != nil {
        log.Fatal(err)
    }
    defer overflow.Flush(2 * time.Second)

    // Capture an error
    _, err = riskyOperation()
    if err != nil {
        overflow.CaptureException(err)
    }

    // Capture a message
    overflow.CaptureMessage("User signed up", overflow.LevelInfo)
}
```

## Configuration

| Option             | Type                  | Default  | Description                               |
| ------------------ | --------------------- | -------- | ----------------------------------------- |
| `DSN`              | `string`              | `""`     | Project DSN from Overflow (empty = no-op) |
| `Environment`      | `string`              | `""`     | Environment name (e.g. `production`)      |
| `Release`          | `string`              | `""`     | Release version (e.g. `myapp@1.0.0`)      |
| `ServerName`       | `string`              | `""`     | Server identifier                         |
| `SampleRate`       | `float64`             | `1.0`    | Event sampling rate (0.0 - 1.0)           |
| `TracesSampleRate` | `float64`             | `0`      | Transaction sampling rate (0.0 - 1.0)     |
| `MaxBreadcrumbs`   | `int`                 | `100`    | Max breadcrumbs to retain                 |
| `BeforeSend`       | `func(*Event) *Event` | `nil`    | Hook to modify/drop events                |
| `Debug`            | `bool`                | `false`  | Enable debug logging                      |
| `User`             | `User`                | `User{}` | Default user context for all events       |
| `Tags`             | `map[string]string`   | `nil`    | Default tags applied to all events        |
| `Contexts`         | `map[string]any`      | `nil`    | Default context objects for all events    |

When `DSN` is empty, the SDK initializes in no-op mode — all capture calls silently succeed without sending data. This is useful for local development where Overflow is intentionally disabled.

## Panic Recovery

Use `Recover` in a top-level defer to capture panics and send them before the process exits:

```go
func main() {
    overflow.Init(overflow.ClientOptions{DSN: "..."})
    defer overflow.Recover()

    panic("something went wrong")
}
```

In middleware, use `RecoverWithRepanic` instead — it captures the panic and then re-panics so the framework's recovery handler can send the HTTP response:

```go
func myMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        defer overflow.RecoverWithRepanic()
        next.ServeHTTP(w, r)
    })
}
```

## HTTP Middleware

### net/http

```go
mux := http.NewServeMux()
mux.HandleFunc("/", handler)
http.ListenAndServe(":8080", overflow.HTTPMiddleware()(mux))
```

The middleware captures panics in HTTP handlers, enriches events with request data, and adds request breadcrumbs. When `TracesSampleRate > 0`, it also creates performance transactions.

### Gin

Install the Gin middleware subpackage:

```bash
go get github.com/Jaggle-AI-HQ/overflow-go/ginoverflow
```

```go
import (
    overflow "github.com/Jaggle-AI-HQ/overflow-go"
    "github.com/Jaggle-AI-HQ/overflow-go/ginoverflow"
)

func main() {
    overflow.Init(overflow.ClientOptions{
        DSN:              "https://<public-key>@your-host.com/api/ingest",
        TracesSampleRate: 1.0,
    })

    r := gin.Default()
    r.Use(ginoverflow.Middleware())
    r.Run()
}
```

The Gin middleware provides the same functionality as `HTTPMiddleware` — panic capture with request context, breadcrumbs, and performance transactions — with no boilerplate.

## Scopes & Context

```go
// Set tags on all future events
overflow.ConfigureScope(func(scope *overflow.Scope) {
    scope.SetTag("component", "payments")
    scope.SetUser(overflow.User{
        ID:    "user-123",
        Email: "user@example.com",
    })
})

// Attach HTTP request context to enrich all events on this scope
overflow.ConfigureScope(func(scope *overflow.Scope) {
    scope.SetRequest(r)
})

// Add breadcrumbs
overflow.AddBreadcrumb(&overflow.Breadcrumb{
    Category: "auth",
    Message:  "User authenticated",
    Level:    "info",
})
```

## Custom Fingerprinting

Override automatic issue grouping:

```go
overflow.ConfigureScope(func(scope *overflow.Scope) {
    scope.SetFingerprint([]string{"payment-failed", "stripe"})
})
```

## Publishing

Releases are triggered by pushing a version tag:

```bash
git tag v0.1.0
git push origin v0.1.0
```

This runs tests, verifies the module, and creates a GitHub Release with auto-generated notes. Go modules are automatically available via `go get` once the tag is pushed.

For pre-release versions, use a semver pre-release suffix (e.g. `v0.2.0-beta.1`).

## Changelog

See [GitHub Releases](https://github.com/Jaggle-AI-HQ/overflow-go/releases).

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup and guidelines.

## Security

See [SECURITY.md](SECURITY.md) for reporting vulnerabilities.

## License

[MIT](LICENSE)
