package overflow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Transport defines how events are delivered to the server.
type Transport interface {
	Send(event *Event)
	Flush(timeout time.Duration) bool
}

// noopTransport silently drops all events. Used when DSN is empty.
type noopTransport struct{}

func (t *noopTransport) Send(event *Event) {}
func (t *noopTransport) Flush(timeout time.Duration) bool { return true }

// HTTPTransport sends events via HTTP POST to the Overflow ingestion API.
type HTTPTransport struct {
	host      string
	publicKey string
	client    *http.Client
	buffer    chan *Event
	wg        sync.WaitGroup
}

// NewHTTPTransport creates a transport that sends events to the given host.
func NewHTTPTransport(host, publicKey string) *HTTPTransport {
	t := &HTTPTransport{
		host:      host,
		publicKey: publicKey,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		buffer: make(chan *Event, 256),
	}
	t.wg.Add(1)
	go t.worker()
	return t
}

// Send queues an event for delivery.
func (t *HTTPTransport) Send(event *Event) {
	select {
	case t.buffer <- event:
	default:
		// Buffer full, drop event
	}
}

// Flush blocks until all queued events are sent or the timeout expires.
func (t *HTTPTransport) Flush(timeout time.Duration) bool {
	done := make(chan struct{})
	go func() {
		t.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return true
	case <-time.After(timeout):
		return false
	}
}

func (t *HTTPTransport) worker() {
	defer t.wg.Done()
	for event := range t.buffer {
		t.sendEvent(event)
	}
}

// SendRaw sends pre-encoded JSON bytes to the ingest endpoint.
func (t *HTTPTransport) SendRaw(body []byte) {
	endpoint := fmt.Sprintf("%s/%s/store", t.host, t.publicKey)
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", fmt.Sprintf("overflow-go/%s", Version))
	resp, err := t.client.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()
}

func (t *HTTPTransport) sendEvent(event *Event) {
	body, err := json.Marshal(event)
	if err != nil {
		return
	}

	endpoint := fmt.Sprintf("%s/%s/store", t.host, t.publicKey)
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", fmt.Sprintf("overflow-go/%s", Version))

	resp, err := t.client.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()
}
