package overflow

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestFlattenHeaders(t *testing.T) {
	h := http.Header{}
	h.Set("Content-Type", "application/json")
	h.Set("Accept", "text/html")
	h.Add("X-Multi", "first")
	h.Add("X-Multi", "second")

	flat := FlattenHeaders(h)

	if flat["Content-Type"] != "application/json" {
		t.Errorf("Content-Type = %q", flat["Content-Type"])
	}
	if flat["Accept"] != "text/html" {
		t.Errorf("Accept = %q", flat["Accept"])
	}
	// Multi-value headers should use the first value
	if flat["X-Multi"] != "first" {
		t.Errorf("X-Multi = %q, want %q", flat["X-Multi"], "first")
	}
}

func TestFlattenHeadersEmpty(t *testing.T) {
	flat := FlattenHeaders(http.Header{})
	if len(flat) != 0 {
		t.Errorf("expected empty map, got %v", flat)
	}
}

func TestStatusWriterDefaultCode(t *testing.T) {
	rec := httptest.NewRecorder()
	sw := &statusWriter{ResponseWriter: rec, code: http.StatusOK}

	// Write without explicit WriteHeader
	sw.Write([]byte("hello"))
	if sw.code != http.StatusOK {
		t.Errorf("default code = %d, want %d", sw.code, http.StatusOK)
	}
}

func TestStatusWriterCapturesCode(t *testing.T) {
	rec := httptest.NewRecorder()
	sw := &statusWriter{ResponseWriter: rec, code: http.StatusOK}

	sw.WriteHeader(http.StatusNotFound)
	if sw.code != http.StatusNotFound {
		t.Errorf("code = %d, want %d", sw.code, http.StatusNotFound)
	}
}

func TestHTTPMiddlewareNormalRequest(t *testing.T) {
	mt := &mockTransport{}
	hub, _ := NewHub(ClientOptions{
		DSN:       "https://testkey@localhost:9999/api/ingest",
		Transport: mt,
	})
	globalHub = hub
	t.Cleanup(func() { globalHub = nil })

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	middleware := HTTPMiddleware()(handler)
	req := httptest.NewRequest("GET", "/api/test", nil)
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.String() != "ok" {
		t.Errorf("body = %q", rec.Body.String())
	}
}

func TestHTTPMiddlewareAddsBreadcrumb(t *testing.T) {
	mt := &mockTransport{}
	hub, _ := NewHub(ClientOptions{
		DSN:       "https://testkey@localhost:9999/api/ingest",
		Transport: mt,
	})
	globalHub = hub
	t.Cleanup(func() { globalHub = nil })

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Capture an event on the request's hub to see the breadcrumbs
		GetHubFromContext(r.Context()).CaptureMessage("test", LevelInfo)
		w.WriteHeader(http.StatusOK)
	})

	middleware := HTTPMiddleware()(handler)
	req := httptest.NewRequest("POST", "/api/users", nil)
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	if len(mt.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(mt.events))
	}
	ev := mt.events[0]
	found := false
	for _, bc := range ev.Breadcrumbs {
		if bc.Type == "http" && bc.Category == "request" && bc.Message == "POST /api/users" {
			found = true
			break
		}
	}
	if !found {
		t.Error("middleware should add HTTP breadcrumb")
	}
}

func TestHTTPMiddlewarePanicRecovery(t *testing.T) {
	mt := &mockTransport{}
	hub, _ := NewHub(ClientOptions{
		DSN:       "https://testkey@localhost:9999/api/ingest",
		Transport: mt,
	})
	globalHub = hub
	t.Cleanup(func() { globalHub = nil })

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("something went wrong")
	})

	middleware := HTTPMiddleware()(handler)
	req := httptest.NewRequest("GET", "/crash", nil)
	rec := httptest.NewRecorder()

	// The middleware re-panics, so we need to recover
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic to be re-raised")
		}
		if r != "something went wrong" {
			t.Errorf("panic value = %v", r)
		}

		// Should have captured the error
		if len(mt.events) != 1 {
			t.Fatalf("expected 1 event, got %d", len(mt.events))
		}
		ev := mt.events[0]
		if ev.Level != LevelFatal {
			t.Errorf("level = %q, want %q", ev.Level, LevelFatal)
		}
		if ev.Message != "something went wrong" {
			t.Errorf("message = %q", ev.Message)
		}
		if ev.Request == nil {
			t.Error("request context should be captured")
		}
	}()

	middleware.ServeHTTP(rec, req)
}

func TestHTTPMiddlewarePanicWithError(t *testing.T) {
	mt := &mockTransport{}
	hub, _ := NewHub(ClientOptions{
		DSN:       "https://testkey@localhost:9999/api/ingest",
		Transport: mt,
	})
	globalHub = hub
	t.Cleanup(func() { globalHub = nil })

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(io.EOF)
	})

	middleware := HTTPMiddleware()(handler)
	req := httptest.NewRequest("GET", "/crash", nil)
	rec := httptest.NewRecorder()

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic to be re-raised")
		}

		if len(mt.events) != 1 {
			t.Fatalf("expected 1 event, got %d", len(mt.events))
		}
		if mt.events[0].Message != "EOF" {
			t.Errorf("message = %q, want %q", mt.events[0].Message, "EOF")
		}
	}()

	middleware.ServeHTTP(rec, req)
}

func TestHTTPMiddlewareWithoutHub(t *testing.T) {
	globalHub = nil

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	middleware := HTTPMiddleware()(handler)
	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d", rec.Code)
	}
}

func TestHTTPMiddlewareWithTracing(t *testing.T) {
	mt := &mockTransport{}
	hub, _ := NewHub(ClientOptions{
		DSN:              "https://testkey@localhost:9999/api/ingest",
		Transport:        mt,
		TracesSampleRate: 1.0,
	})
	globalHub = hub
	t.Cleanup(func() { globalHub = nil })

	var txnFromCtx *Transaction
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		txnFromCtx = TransactionFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	middleware := HTTPMiddleware()(handler)
	req := httptest.NewRequest("GET", "/api/data", nil)
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	if txnFromCtx == nil {
		t.Fatal("transaction should be available in request context")
	}
	if txnFromCtx.name != "GET /api/data" {
		t.Errorf("transaction name = %q", txnFromCtx.name)
	}
	if txnFromCtx.op != "http.server" {
		t.Errorf("transaction op = %q", txnFromCtx.op)
	}
}

// syncTransport records events and is safe for concurrent use.
type syncTransport struct {
	mu     sync.Mutex
	events []*Event
}

func (m *syncTransport) Send(event *Event) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, event)
}

func (m *syncTransport) Flush(timeout time.Duration) bool { return true }

func TestHTTPMiddlewareKeepsRequestDataOffGlobalScope(t *testing.T) {
	mt := &syncTransport{}
	hub, _ := NewHub(ClientOptions{
		DSN:       "https://testkey@localhost:9999/api/ingest",
		Transport: mt,
	})
	globalHub = hub
	t.Cleanup(func() { globalHub = nil })

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqHub := GetHubFromContext(r.Context())
		if reqHub == nil || reqHub == hub {
			t.Error("request should carry its own hub")
			return
		}
		reqHub.Scope().SetTag("path", r.URL.Path)
		reqHub.CaptureMessage(r.URL.Path, LevelInfo)
	})
	middleware := HTTPMiddleware()(handler)

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			req := httptest.NewRequest("GET", fmt.Sprintf("/r/%d", i), nil)
			middleware.ServeHTTP(httptest.NewRecorder(), req)
		}(i)
	}
	wg.Wait()

	mt.mu.Lock()
	events := append([]*Event(nil), mt.events...)
	mt.mu.Unlock()
	if len(events) != 20 {
		t.Fatalf("expected 20 events, got %d", len(events))
	}
	for _, ev := range events {
		if ev.Tags["path"] != ev.Message {
			t.Errorf("event %q has tag path=%q from another request", ev.Message, ev.Tags["path"])
		}
		if len(ev.Breadcrumbs) != 1 || ev.Breadcrumbs[0].Message != "GET "+ev.Message {
			t.Errorf("event %q has breadcrumbs %+v, want only its own request", ev.Message, ev.Breadcrumbs)
		}
	}

	CaptureMessage("global", LevelInfo)
	mt.mu.Lock()
	global := mt.events[len(mt.events)-1]
	mt.mu.Unlock()
	if len(global.Breadcrumbs) != 0 || global.Tags["path"] != "" {
		t.Errorf("global scope picked up request data: tags=%v breadcrumbs=%v", global.Tags, global.Breadcrumbs)
	}
}

// slowFlushTransport records events and blocks in Flush, like an HTTP
// transport whose server is slow.
type slowFlushTransport struct {
	syncTransport
	flushes int
}

func (m *slowFlushTransport) Flush(timeout time.Duration) bool {
	m.mu.Lock()
	m.flushes++
	m.mu.Unlock()
	time.Sleep(timeout)
	return false
}

func TestHTTPMiddlewarePanicDoesNotWaitForDelivery(t *testing.T) {
	mt := &slowFlushTransport{}
	hub, _ := NewHub(ClientOptions{
		DSN:       "https://testkey@localhost:9999/api/ingest",
		Transport: mt,
	})
	globalHub = hub
	t.Cleanup(func() { globalHub = nil })

	middleware := HTTPMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}))

	start := time.Now()
	func() {
		defer func() { recover() }()
		middleware.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/crash", nil))
	}()

	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Errorf("panicking request took %v", elapsed)
	}
	if mt.flushes != 0 {
		t.Errorf("middleware flushed %d times, want 0", mt.flushes)
	}
	if len(mt.events) != 1 {
		t.Errorf("expected 1 event, got %d", len(mt.events))
	}
}
