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

| Option           | Type                  | Default  | Description                          |
| ---------------- | --------------------- | -------- | ------------------------------------ |
| `DSN`            | `string`              | required | Project DSN from Overflow            |
| `Environment`    | `string`              | `""`     | Environment name (e.g. `production`) |
| `Release`        | `string`              | `""`     | Release version (e.g. `myapp@1.0.0`) |
| `ServerName`     | `string`              | `""`     | Server identifier                    |
| `SampleRate`     | `float64`             | `1.0`    | Event sampling rate (0.0 - 1.0)      |
| `MaxBreadcrumbs` | `int`                 | `100`    | Max breadcrumbs to retain            |
| `BeforeSend`     | `func(*Event) *Event` | `nil`    | Hook to modify/drop events           |
| `Debug`          | `bool`                | `false`  | Enable debug logging                 |

## Panic Recovery

```go
func main() {
    overflow.Init(overflow.ClientOptions{DSN: "..."})
    defer overflow.Recover()

    panic("something went wrong")
}
```

## HTTP Middleware

```go
mux := http.NewServeMux()
mux.HandleFunc("/", handler)
http.ListenAndServe(":8080", overflow.HTTPMiddleware()(mux))
```

The middleware captures panics in HTTP handlers, enriches events with request data, and adds request breadcrumbs.

## Scopes & Context

```go
// Set tags on all future events
overflow.ConfigureScope(func(scope *overflow.Scope) {
    scope.SetTag("component", "payments")
    scope.SetUser(map[string]any{
        "id":    "user-123",
        "email": "user@example.com",
    })
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
