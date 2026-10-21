package overflow

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNoopTransport(t *testing.T) {
	tr := &noopTransport{}
	// Should not panic
	tr.Send(&Event{Message: "test"})
	if !tr.Flush(time.Second) {
		t.Error("noopTransport Flush should return true")
	}
}

func TestHTTPTransportSendAndFlush(t *testing.T) {
	var received *Event
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var ev Event
		json.Unmarshal(body, &ev)
		received = &ev
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tr := NewHTTPTransport(server.URL, "testkey")
	event := NewEvent()
	event.Message = "hello"
	tr.Send(event)

	// Close the queues to let the workers finish
	closeTransport(tr)

	if received == nil {
		t.Fatal("server should have received an event")
	}
	if received.Message != "hello" {
		t.Errorf("received message = %q, want %q", received.Message, "hello")
	}
}

func TestHTTPTransportSendHeaders(t *testing.T) {
	var gotContentType, gotUserAgent string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		gotUserAgent = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tr := NewHTTPTransport(server.URL, "testkey")
	tr.Send(NewEvent())
	closeTransport(tr)

	if gotContentType != "application/json" {
		t.Errorf("Content-Type = %q, want %q", gotContentType, "application/json")
	}
	expectedUA := "overflow-go/" + Version
	if gotUserAgent != expectedUA {
		t.Errorf("User-Agent = %q, want %q", gotUserAgent, expectedUA)
	}
}

func TestHTTPTransportEndpoint(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tr := NewHTTPTransport(server.URL, "mykey123")
	tr.Send(NewEvent())
	closeTransport(tr)

	if gotPath != "/mykey123/store" {
		t.Errorf("path = %q, want %q", gotPath, "/mykey123/store")
	}
}

func TestHTTPTransportBufferFull(t *testing.T) {
	// Create transport pointing to a server that won't process fast
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tr := &HTTPTransport{
		host:      server.URL,
		publicKey: "key",
		client:    &http.Client{Timeout: 5 * time.Second},
		buffer:    make(chan *Event, 2), // tiny buffer
	}

	// Fill the buffer
	tr.Send(NewEvent())
	tr.Send(NewEvent())

	// This should silently drop (buffer full, no worker draining)
	tr.Send(NewEvent())
}

func TestHTTPTransportSendRaw(t *testing.T) {
	var gotBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tr := NewHTTPTransport(server.URL, "rawkey")

	payload := []byte(`{"type":"transaction","trace_id":"abc123"}`)
	tr.SendRaw(payload)

	if string(gotBody) != string(payload) {
		t.Errorf("body = %q, want %q", gotBody, payload)
	}

	// Clean up workers
	closeTransport(tr)
}

func TestHTTPTransportFlushTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tr := NewHTTPTransport(server.URL, "key")
	tr.Send(NewEvent())

	// Flush with very short timeout should return false since the event is still being processed
	result := tr.Flush(1 * time.Millisecond)
	// Note: The result depends on timing - the worker may or may not have started.
	// We just verify it doesn't panic.
	_ = result

	closeTransport(tr)
}

// closeTransport stops the transport's workers after they drain their queues.
func closeTransport(tr *HTTPTransport) {
	close(tr.buffer)
	close(tr.raw)
	tr.wg.Wait()
}

func TestHTTPTransportFlushReturnsWhenIdle(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tr := NewHTTPTransport(server.URL, "key")
	defer closeTransport(tr)

	start := time.Now()
	if !tr.Flush(2 * time.Second) {
		t.Fatal("Flush with nothing queued should return true")
	}
	if time.Since(start) > 100*time.Millisecond {
		t.Errorf("Flush with nothing queued took %v", time.Since(start))
	}
}

func TestHTTPTransportFlushWaitsForInFlightSends(t *testing.T) {
	var received atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		received.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tr := NewHTTPTransport(server.URL, "key")
	defer closeTransport(tr)

	tr.Send(NewEvent())
	tr.Send(NewEvent())
	tr.sendRawAsync([]byte(`{"type":"transaction"}`))

	start := time.Now()
	if !tr.Flush(2 * time.Second) {
		t.Fatal("Flush should return true once the sends finish")
	}
	if got := received.Load(); got != 3 {
		t.Errorf("server received %d requests before Flush returned, want 3", got)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("Flush took %v, want it to return when the sends finish", elapsed)
	}

	// A second Flush after everything is delivered returns immediately.
	start = time.Now()
	if !tr.Flush(2 * time.Second) {
		t.Fatal("second Flush should return true")
	}
	if time.Since(start) > 100*time.Millisecond {
		t.Errorf("second Flush took %v", time.Since(start))
	}
}

func TestHTTPTransportFlushTimesOutWhileSending(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	defer close(release)

	tr := NewHTTPTransport(server.URL, "key")
	tr.Send(NewEvent())

	if tr.Flush(50 * time.Millisecond) {
		t.Error("Flush should time out while a send is blocked")
	}
}

func TestHTTPTransportDroppedEventDoesNotBlockFlush(t *testing.T) {
	tr := &HTTPTransport{
		host:      "http://127.0.0.1:0",
		publicKey: "key",
		client:    &http.Client{Timeout: time.Second},
		buffer:    make(chan *Event), // unbuffered and never drained
	}

	tr.Send(NewEvent())
	if tr.sendRawAsync([]byte(`{}`)) {
		t.Error("sendRawAsync without a raw queue should report a drop")
	}
	if !tr.Flush(10 * time.Millisecond) {
		t.Error("dropped items must not count as pending")
	}
}

func TestHTTPTransportRawQueueDropsWhenFull(t *testing.T) {
	tr := &HTTPTransport{
		host:      "http://127.0.0.1:0",
		publicKey: "key",
		client:    &http.Client{Timeout: time.Second},
		raw:       make(chan []byte, 2), // no worker draining
	}

	if !tr.sendRawAsync([]byte(`1`)) || !tr.sendRawAsync([]byte(`2`)) {
		t.Fatal("first two payloads should be queued")
	}
	if tr.sendRawAsync([]byte(`3`)) {
		t.Error("third payload should be dropped when the queue is full")
	}
	if tr.pending != 2 {
		t.Errorf("pending = %d, want 2", tr.pending)
	}
}

func TestHTTPTransportReusesConnections(t *testing.T) {
	var mu sync.Mutex
	conns := map[string]bool{}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		conns[r.RemoteAddr] = true
		mu.Unlock()
		// A body larger than the client's read-ahead, so it is only reusable when drained.
		w.Write(bytes.Repeat([]byte("x"), 32<<10))
	}))
	server.Start()
	defer server.Close()

	tr := NewHTTPTransport(server.URL, "key")
	defer closeTransport(tr)

	for i := 0; i < 5; i++ {
		tr.SendRaw([]byte(`{}`))
	}

	mu.Lock()
	defer mu.Unlock()
	if len(conns) != 1 {
		t.Errorf("used %d connections for 5 sequential sends, want 1", len(conns))
	}
}
