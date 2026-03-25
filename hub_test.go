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
	id := hub.CaptureExceptionWithRequest(errors.New("request error"), req, LevelError)

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

func TestHubCaptureExceptionWithRequestLevel(t *testing.T) {
	mt := &mockTransport{}
	hub, _ := NewHub(ClientOptions{
		DSN:       "https://testkey@localhost:9999/api/ingest",
		Transport: mt,
	})

	req, _ := http.NewRequest("GET", "http://example.com/missing", nil)
	hub.CaptureExceptionWithRequest(errors.New("not found"), req, LevelWarning)

	ev := mt.events[0]
	if ev.Level != LevelWarning {
		t.Errorf("level = %q, want %q", ev.Level, LevelWarning)
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

func TestHubAppliesUserFromOptions(t *testing.T) {
	mt := &mockTransport{}
	hub, _ := NewHub(ClientOptions{
		DSN:       "https://testkey@localhost:9999/api/ingest",
		Transport: mt,
		User:      User{ID: "u-1", Email: "alice@example.com"},
	})

	hub.CaptureMessage("test", LevelInfo)
	ev := mt.events[0]

	if ev.User == nil {
		t.Fatal("User should not be nil")
	}
	if ev.User.ID != "u-1" {
		t.Errorf("User.ID = %q, want %q", ev.User.ID, "u-1")
	}
	if ev.User.Email != "alice@example.com" {
		t.Errorf("User.Email = %q", ev.User.Email)
	}
}

func TestHubAppliesTagsFromOptions(t *testing.T) {
	mt := &mockTransport{}
	hub, _ := NewHub(ClientOptions{
		DSN:       "https://testkey@localhost:9999/api/ingest",
		Transport: mt,
		Tags:      map[string]string{"service": "api", "region": "us-east"},
	})

	hub.CaptureMessage("test", LevelInfo)
	ev := mt.events[0]

	if ev.Tags["service"] != "api" {
		t.Errorf("Tags[service] = %q", ev.Tags["service"])
	}
	if ev.Tags["region"] != "us-east" {
		t.Errorf("Tags[region] = %q", ev.Tags["region"])
	}
}

func TestHubAppliesContextsFromOptions(t *testing.T) {
	mt := &mockTransport{}
	hub, _ := NewHub(ClientOptions{
		DSN:       "https://testkey@localhost:9999/api/ingest",
		Transport: mt,
		Contexts:  map[string]any{"device": map[string]any{"arch": "arm64"}},
	})

	hub.CaptureMessage("test", LevelInfo)
	ev := mt.events[0]

	ctx, ok := ev.Contexts["device"].(map[string]any)
	if !ok {
		t.Fatal("device context should be set")
	}
	if ctx["arch"] != "arm64" {
		t.Errorf("arch = %v", ctx["arch"])
	}
}

func TestHubOptionDefaultsOverriddenByScope(t *testing.T) {
	mt := &mockTransport{}
	hub, _ := NewHub(ClientOptions{
		DSN:       "https://testkey@localhost:9999/api/ingest",
		Transport: mt,
		User:      User{ID: "default-user"},
		Tags:      map[string]string{"env": "default"},
	})

	hub.Scope().SetUser(User{ID: "override-user"})
	hub.Scope().SetTag("env", "override")

	hub.CaptureMessage("test", LevelInfo)
	ev := mt.events[0]

	if ev.User == nil || ev.User.ID != "override-user" {
		t.Errorf("User should be overridden, got %v", ev.User)
	}
	if ev.Tags["env"] != "override" {
		t.Errorf("Tags[env] = %q, want %q", ev.Tags["env"], "override")
	}
}
