# overflow-go

Official Go SDK for [Overflow](https://github.com/Jaggle-AI-HQ/jaggle-overflow) error tracking.

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

## Version

Current SDK version: `0.1.0`

## License

MIT
