package overflow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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

func (t *noopTransport) Send(event *Event)                {}
func (t *noopTransport) Flush(timeout time.Duration) bool { return true }

// transportBufferSize is the queue length for events and for transactions.
// Items sent while a queue is full are dropped.
const transportBufferSize = 256

// maxDrainBytes caps how much of a response body is read so the connection
// can go back to the keep-alive pool.
const maxDrainBytes = 64 << 10

// HTTPTransport sends events via HTTP POST to the Overflow ingestion API.
// Events and transactions are queued and delivered by background workers;
// when a queue is full, new items are dropped instead of blocking the caller.
type HTTPTransport struct {
	host      string
	publicKey string
	client    *http.Client
	buffer    chan *Event
	raw       chan []byte
	wg        sync.WaitGroup

	// pending counts queued and in-progress deliveries. idle is closed while
	// pending is zero, so Flush can wait on it.
	mu      sync.Mutex
	pending int
	idle    chan struct{}
}

// NewHTTPTransport creates a transport that sends events to the given host.
func NewHTTPTransport(host, publicKey string) *HTTPTransport {
	t := &HTTPTransport{
		host:      host,
		publicKey: publicKey,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		buffer: make(chan *Event, transportBufferSize),
		raw:    make(chan []byte, transportBufferSize),
	}
	t.wg.Add(2)
	go t.worker()
	go t.rawWorker()
	return t
}

// Send queues an event for delivery.
func (t *HTTPTransport) Send(event *Event) {
	t.begin()
	select {
	case t.buffer <- event:
	default:
		// Buffer full, drop event
		t.done()
	}
}

// sendRawAsync queues pre-encoded JSON for delivery and reports whether it was
// accepted. It drops the payload when the queue is full.
func (t *HTTPTransport) sendRawAsync(body []byte) bool {
	if t.raw == nil {
		return false
	}
	t.begin()
	select {
	case t.raw <- body:
		return true
	default:
		t.done()
		return false
	}
}

// Flush blocks until all queued events and transactions are sent or the
// timeout expires. It returns true when nothing is left to send.
func (t *HTTPTransport) Flush(timeout time.Duration) bool {
	t.mu.Lock()
	if t.pending == 0 {
		t.mu.Unlock()
		return true
	}
	idle := t.idle
	t.mu.Unlock()

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-idle:
		return true
	case <-timer.C:
		return false
	}
}

func (t *HTTPTransport) begin() {
	t.mu.Lock()
	if t.pending == 0 || t.idle == nil {
		t.idle = make(chan struct{})
	}
	t.pending++
	t.mu.Unlock()
}

func (t *HTTPTransport) done() {
	t.mu.Lock()
	t.pending--
	if t.pending == 0 && t.idle != nil {
		close(t.idle)
	}
	t.mu.Unlock()
}

func (t *HTTPTransport) worker() {
	defer t.wg.Done()
	for event := range t.buffer {
		t.sendEvent(event)
		t.done()
	}
}

func (t *HTTPTransport) rawWorker() {
	defer t.wg.Done()
	for body := range t.raw {
		t.SendRaw(body)
		t.done()
	}
}

// SendRaw sends pre-encoded JSON bytes to the ingest endpoint. It blocks
// until the request completes.
func (t *HTTPTransport) SendRaw(body []byte) {
	t.post(body)
}

func (t *HTTPTransport) sendEvent(event *Event) {
	body, err := json.Marshal(event)
	if err != nil {
		return
	}
	t.post(body)
}

func (t *HTTPTransport) post(body []byte) {
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
	// Read the body so the keep-alive connection is reused.
	io.Copy(io.Discard, io.LimitReader(resp.Body, maxDrainBytes))
	resp.Body.Close()
}
