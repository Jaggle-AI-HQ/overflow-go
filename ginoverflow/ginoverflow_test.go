package ginoverflow

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/Jaggle-AI-HQ/overflow-go"
	"github.com/gin-gonic/gin"
)

// recordingTransport records events and counts Flush calls.
type recordingTransport struct {
	mu      sync.Mutex
	events  []*overflow.Event
	flushes int
}

func (m *recordingTransport) Send(event *overflow.Event) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, event)
}

func (m *recordingTransport) Flush(timeout time.Duration) bool {
	m.mu.Lock()
	m.flushes++
	m.mu.Unlock()
	time.Sleep(timeout)
	return false
}

func (m *recordingTransport) snapshot() ([]*overflow.Event, int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]*overflow.Event(nil), m.events...), m.flushes
}

func setup(t *testing.T) *recordingTransport {
	t.Helper()
	gin.SetMode(gin.TestMode)
	mt := &recordingTransport{}
	if err := overflow.Init(overflow.ClientOptions{
		DSN:       "https://testkey@localhost:9999/api/ingest",
		Transport: mt,
	}); err != nil {
		t.Fatal(err)
	}
	return mt
}

func TestMiddlewareUsesPerRequestHub(t *testing.T) {
	mt := setup(t)

	r := gin.New()
	r.Use(Middleware())
	r.GET("/r/:id", func(c *gin.Context) {
		hub := overflow.GetHubFromContext(c.Request.Context())
		if hub == nil || hub == overflow.GetHub() {
			t.Error("request should carry its own hub")
			return
		}
		hub.CaptureMessage(c.Request.URL.Path, overflow.LevelInfo)
	})

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", fmt.Sprintf("/r/%d", i), nil))
		}(i)
	}
	wg.Wait()

	events, _ := mt.snapshot()
	if len(events) != 20 {
		t.Fatalf("expected 20 events, got %d", len(events))
	}
	for _, ev := range events {
		if got := ev.Request["url"]; got != ev.Message {
			t.Errorf("event %q carries request url %v", ev.Message, got)
		}
		if len(ev.Breadcrumbs) != 1 || ev.Breadcrumbs[0].Message != "GET "+ev.Message {
			t.Errorf("event %q has breadcrumbs %+v, want only its own request", ev.Message, ev.Breadcrumbs)
		}
	}

	overflow.CaptureMessage("global", overflow.LevelInfo)
	events, _ = mt.snapshot()
	global := events[len(events)-1]
	if global.Request != nil || len(global.Breadcrumbs) != 0 {
		t.Errorf("global scope picked up request data: request=%v breadcrumbs=%v", global.Request, global.Breadcrumbs)
	}
}

func TestMiddlewarePanicDoesNotWaitForDelivery(t *testing.T) {
	mt := setup(t)

	r := gin.New()
	r.Use(gin.Recovery(), Middleware())
	r.GET("/crash", func(c *gin.Context) { panic("boom") })

	rec := httptest.NewRecorder()
	start := time.Now()
	r.ServeHTTP(rec, httptest.NewRequest("GET", "/crash", nil))

	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Errorf("panicking request took %v", elapsed)
	}
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	events, flushes := mt.snapshot()
	if flushes != 0 {
		t.Errorf("middleware flushed %d times, want 0", flushes)
	}
	if len(events) != 1 || events[0].Message != "boom" {
		t.Errorf("events = %+v", events)
	}
}
