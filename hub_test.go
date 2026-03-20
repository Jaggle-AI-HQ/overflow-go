package overflow

import (
	"errors"
	"net/http"
	"testing"
)

func TestNewHub(t *testing.T) {
	hub, err := NewHub(ClientOptions{
		DSN:       "https://testkey@localhost:9999/api/ingest",
		Transport: &mockTransport{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hub.Client() == nil {
		t.Error("Client should not be nil")
	}
	if hub.Scope() == nil {
		t.Error("Scope should not be nil")
	}
}

func TestNewHubInvalidDSN(t *testing.T) {
	_, err := NewHub(ClientOptions{DSN: "https://example.com/no-key"})
	if err == nil {
		t.Fatal("expected error for invalid DSN")
	}
}

func TestHubCaptureException(t *testing.T) {
	mt := &mockTransport{}
	hub, _ := NewHub(ClientOptions{
		DSN:       "https://testkey@localhost:9999/api/ingest",
		Transport: mt,
	})

	id := hub.CaptureException(errors.New("test error"))
	if id == "" {
		t.Fatal("expected event ID")
	}
	if len(mt.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(mt.events))
	}

	ev := mt.events[0]
	if ev.Level != LevelError {
		t.Errorf("level = %q, want %q", ev.Level, LevelError)
	}
	if ev.Message != "test error" {
		t.Errorf("message = %q", ev.Message)
	}
	if ev.Exception == nil {
		t.Error("exception should not be nil")
	}
}

func TestHubCaptureExceptionWithRequest(t *testing.T) {
	mt := &mockTransport{}
	hub, _ := NewHub(ClientOptions{
		DSN:       "https://testkey@localhost:9999/api/ingest",
		Transport: mt,
	})

	req, _ := http.NewRequest("POST", "http://example.com/api/test", nil)
	req.Header.Set("Content-Type", "application/json")
	id := hub.CaptureExceptionWithRequest(errors.New("request error"), req)

	if id == "" {
		t.Fatal("expected event ID")
	}
	ev := mt.events[0]
	if ev.Request == nil {
		t.Fatal("request should not be nil")
	}
	if ev.Request["method"] != "POST" {
		t.Errorf("method = %v, want POST", ev.Request["method"])
	}
	if ev.Request["url"] != "http://example.com/api/test" {
		t.Errorf("url = %v", ev.Request["url"])
	}
	headers, ok := ev.Request["headers"].(map[string]string)
	if !ok {
		t.Fatal("headers should be map[string]string")
	}
	if headers["Content-Type"] != "application/json" {
		t.Errorf("Content-Type header = %q", headers["Content-Type"])
	}
}

func TestHubCaptureMessage(t *testing.T) {
	mt := &mockTransport{}
	hub, _ := NewHub(ClientOptions{
		DSN:       "https://testkey@localhost:9999/api/ingest",
		Transport: mt,
	})

	id := hub.CaptureMessage("hello", LevelInfo)
	if id == "" {
		t.Fatal("expected event ID")
	}
	ev := mt.events[0]
	if ev.Message != "hello" {
		t.Errorf("message = %q", ev.Message)
	}
	if ev.Level != LevelInfo {
		t.Errorf("level = %q, want %q", ev.Level, LevelInfo)
	}
}

func TestHubAddBreadcrumb(t *testing.T) {
	mt := &mockTransport{}
	hub, _ := NewHub(ClientOptions{
		DSN:       "https://testkey@localhost:9999/api/ingest",
		Transport: mt,
	})

	hub.AddBreadcrumb(&Breadcrumb{
		Message:  "user clicked button",
		Category: "ui",
	})

	hub.CaptureMessage("test", LevelInfo)

	ev := mt.events[0]
	if len(ev.Breadcrumbs) != 1 {
		t.Fatalf("expected 1 breadcrumb, got %d", len(ev.Breadcrumbs))
	}
	if ev.Breadcrumbs[0].Message != "user clicked button" {
		t.Errorf("breadcrumb message = %q", ev.Breadcrumbs[0].Message)
	}
}

func TestHubAppliesOptionsToEvents(t *testing.T) {
	mt := &mockTransport{}
	hub, _ := NewHub(ClientOptions{
		DSN:         "https://testkey@localhost:9999/api/ingest",
		Transport:   mt,
		Environment: "staging",
		Release:     "v2.0.0",
		ServerName:  "worker-01",
	})

	hub.CaptureMessage("test", LevelInfo)
	ev := mt.events[0]

	if ev.Environment != "staging" {
		t.Errorf("Environment = %q", ev.Environment)
	}
	if ev.Release != "v2.0.0" {
		t.Errorf("Release = %q", ev.Release)
	}
	if ev.ServerName != "worker-01" {
		t.Errorf("ServerName = %q", ev.ServerName)
	}
}
