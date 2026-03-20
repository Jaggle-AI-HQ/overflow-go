// Package overflow provides error tracking and event capture for Go applications.
//
// Initialize the SDK with [Init] and capture errors with [CaptureException]
// or messages with [CaptureMessage]. The SDK sends events to an Overflow
// server using the project's DSN.
//
//	err := overflow.Init(overflow.ClientOptions{
//	    DSN:         "https://<public-key>@<host>/api/ingest",
//	    Environment: "production",
//	    Release:     "myapp@1.0.0",
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer overflow.Flush(2 * time.Second)
//
//	overflow.CaptureException(someError)
package overflow

import (
	"fmt"
	"net/http"
	"net/url"
	"time"
)

const Version = "0.1.0"

var globalHub *Hub

// ClientOptions configures the Overflow SDK.
type ClientOptions struct {
	// DSN is the Data Source Name for your project.
	// Format: https://<public-key>@<host>/api/ingest
	DSN string

	// Environment sets the environment tag (e.g. "production", "staging").
	Environment string

	// Release sets the release version (e.g. "myapp@1.0.0").
	Release string

	// ServerName identifies this server instance.
	ServerName string

	// SampleRate controls the percentage of events sent (0.0 to 1.0). Default: 1.0.
	SampleRate float64

	// MaxBreadcrumbs is the maximum number of breadcrumbs to keep. Default: 100.
	MaxBreadcrumbs int

	// BeforeSend is called before each event is sent. Return nil to drop the event.
	BeforeSend func(event *Event) *Event

	// Transport overrides the default HTTP transport.
	Transport Transport

	// Debug enables debug logging to stdout.
	Debug bool

	// TracesSampleRate controls the percentage of transactions captured (0.0 to 1.0). Default: 0 (disabled).
	TracesSampleRate float64
}

// Init initializes the global Overflow SDK client.
func Init(options ClientOptions) error {
	hub, err := NewHub(options)
	if err != nil {
		return err
	}
	globalHub = hub
	return nil
}

// CaptureException captures an error and sends it to Overflow.
// Returns the event ID or empty string if the event was dropped.
func CaptureException(err error) string {
	hub := GetHub()
	if hub == nil {
		return ""
	}
	return hub.CaptureException(err)
}

// CaptureMessage captures a message and sends it to Overflow.
func CaptureMessage(msg string, level Level) string {
	hub := GetHub()
	if hub == nil {
		return ""
	}
	return hub.CaptureMessage(msg, level)
}

// CaptureExceptionWithRequest captures an error with HTTP request context and sends it to Overflow.
func CaptureExceptionWithRequest(err error, r *http.Request) string {
	hub := GetHub()
	if hub == nil {
		return ""
	}
	return hub.CaptureExceptionWithRequest(err, r)
}

// AddBreadcrumb adds a breadcrumb to the current scope.
func AddBreadcrumb(breadcrumb *Breadcrumb) {
	hub := GetHub()
	if hub == nil {
		return
	}
	hub.AddBreadcrumb(breadcrumb)
}

// ConfigureScope lets you modify the current scope.
func ConfigureScope(fn func(scope *Scope)) {
	hub := GetHub()
	if hub == nil {
		return
	}
	fn(hub.Scope())
}

// Flush waits until all events have been sent or the timeout is reached.
func Flush(timeout time.Duration) bool {
	hub := GetHub()
	if hub == nil {
		return true
	}
	return hub.Client().Flush(timeout)
}

// Recover captures a panic value and sends it as an event.
// Use in a deferred call:
//
//	defer overflow.Recover()
func Recover() {
	if r := recover(); r != nil {
		hub := GetHub()
		if hub == nil {
			return
		}
		var err error
		switch v := r.(type) {
		case error:
			err = v
		default:
			err = fmt.Errorf("%v", v)
		}
		hub.CaptureException(err)
		hub.Client().Flush(2 * time.Second)
	}
}

// RecoverWithRepanic captures a panic value, sends it as an event, and then
// re-panics so the framework's recovery handler can respond. Use in middleware:
//
//	defer overflow.RecoverWithRepanic()
func RecoverWithRepanic() {
	if r := recover(); r != nil {
		hub := GetHub()
		if hub != nil {
			var err error
			switch v := r.(type) {
			case error:
				err = v
			default:
				err = fmt.Errorf("%v", v)
			}
			hub.CaptureException(err)
			hub.Client().Flush(2 * time.Second)
		}
		panic(r)
	}
}

// GetHub returns the global hub instance.
func GetHub() *Hub {
	return globalHub
}

// parseDSN extracts host and public key from a DSN string.
// Format: https://<public-key>@<host>/api/ingest
func parseDSN(dsn string) (host, publicKey string, err error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return "", "", fmt.Errorf("overflow: invalid DSN: %w", err)
	}
	if u.User == nil || u.User.Username() == "" {
		return "", "", fmt.Errorf("overflow: DSN missing public key")
	}
	publicKey = u.User.Username()
	u.User = nil
	host = u.String()
	return host, publicKey, nil
}
