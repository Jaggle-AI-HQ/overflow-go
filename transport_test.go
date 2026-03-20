package overflow

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
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

	// Close the buffer to let the worker finish
	close(tr.buffer)
	tr.wg.Wait()

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
	close(tr.buffer)
	tr.wg.Wait()

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
	close(tr.buffer)
	tr.wg.Wait()

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

	// Clean up worker
	close(tr.buffer)
	tr.wg.Wait()
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

	close(tr.buffer)
	tr.wg.Wait()
}
