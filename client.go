package overflow

import (
	"fmt"
	"math/rand"
	"time"
)

// Client manages event delivery to the Overflow server.
type Client struct {
	options   ClientOptions
	transport Transport
}

// NewClient creates a new client from the given options.
// If DSN is empty, it returns a no-op client that silently drops all events.
// This allows the SDK to be safely initialized in environments (like local dev)
// where overflow is intentionally disabled.
func NewClient(options ClientOptions) (*Client, error) {
	if options.DSN == "" {
		if options.Debug {
			fmt.Println("[overflow] DSN is empty, client will operate in no-op mode")
		}
		return &Client{
			options:   options,
			transport: &noopTransport{},
		}, nil
	}
	if options.SampleRate == 0 {
		options.SampleRate = 1.0
	}
	if options.MaxBreadcrumbs == 0 {
		options.MaxBreadcrumbs = 100
	}

	var transport Transport
	if options.Transport != nil {
		transport = options.Transport
	} else {
		host, publicKey, err := parseDSN(options.DSN)
		if err != nil {
			return nil, err
		}
		transport = NewHTTPTransport(host, publicKey)
	}

	return &Client{
		options:   options,
		transport: transport,
	}, nil
}

// Send delivers an event to the server. Returns the event ID or empty string if dropped.
func (c *Client) Send(event *Event) string {
	// Sample rate check
	if c.options.SampleRate < 1.0 && rand.Float64() > c.options.SampleRate {
		return ""
	}

	// BeforeSend hook
	if c.options.BeforeSend != nil {
		event = c.options.BeforeSend(event)
		if event == nil {
			return ""
		}
	}

	if c.options.Debug {
		fmt.Printf("[overflow] sending event %s\n", event.EventID)
	}

	c.transport.Send(event)
	return event.EventID
}

// Flush waits for all pending events to be sent.
func (c *Client) Flush(timeout time.Duration) bool {
	return c.transport.Flush(timeout)
}

// Options returns the client's configuration.
func (c *Client) Options() ClientOptions {
	return c.options
}

// applyOptions sets client-level fields on the event.
func (c *Client) applyOptions(event *Event) {
	if c.options.Environment != "" && event.Environment == "" {
		event.Environment = c.options.Environment
	}
	if c.options.Release != "" && event.Release == "" {
		event.Release = c.options.Release
	}
	if c.options.ServerName != "" && event.ServerName == "" {
		event.ServerName = c.options.ServerName
	}
}
