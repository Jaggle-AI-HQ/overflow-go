package overflow

import (
	"errors"
	"net/http"
	"testing"
	"time"
)

func TestParseDSN(t *testing.T) {
	tests := []struct {
		name      string
		dsn       string
		wantHost  string
		wantKey   string
		wantError bool
	}{
		{
			name:     "valid DSN",
			dsn:      "https://abc123@example.com/api/ingest",
			wantHost: "https://example.com/api/ingest",
			wantKey:  "abc123",
		},
		{
			name:     "valid DSN with port",
			dsn:      "https://mykey@localhost:9000/api/ingest",
			wantHost: "https://localhost:9000/api/ingest",
			wantKey:  "mykey",
		},
		{
			name:      "missing public key",
			dsn:       "https://example.com/api/ingest",
			wantError: true,
		},
		{
			name:      "invalid URL",
			dsn:       "://bad",
			wantError: true,
		},
		{
			name:      "empty DSN",
			dsn:       "",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, key, err := parseDSN(tt.dsn)
			if tt.wantError {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if host != tt.wantHost {
				t.Errorf("host = %q, want %q", host, tt.wantHost)
			}
			if key != tt.wantKey {
				t.Errorf("key = %q, want %q", key, tt.wantKey)
			}
		})
	}
}

func TestInitWithEmptyDSN(t *testing.T) {
	globalHub = nil
	err := Init(ClientOptions{})
	if err != nil {
		t.Fatalf("Init with empty DSN should succeed: %v", err)
	}
	if GetHub() == nil {
		t.Fatal("global hub should be set after Init")
	}
	t.Cleanup(func() { globalHub = nil })
}

func TestInitWithValidDSN(t *testing.T) {
	globalHub = nil
	err := Init(ClientOptions{
		DSN:         "https://testkey@localhost:9999/api/ingest",
		Environment: "test",
	})
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	if GetHub() == nil {
		t.Fatal("global hub should be set")
	}
	t.Cleanup(func() { globalHub = nil })
}

func TestInitWithInvalidDSN(t *testing.T) {
	globalHub = nil
	err := Init(ClientOptions{DSN: "https://example.com/no-key"})
	if err == nil {
		t.Fatal("Init with invalid DSN should fail")
	}
	t.Cleanup(func() { globalHub = nil })
}

func TestCaptureExceptionWithoutInit(t *testing.T) {
	globalHub = nil
	id := CaptureException(errors.New("test"))
	if id != "" {
		t.Errorf("CaptureException without init should return empty string, got %q", id)
	}
}

func TestCaptureMessageWithoutInit(t *testing.T) {
	globalHub = nil
	id := CaptureMessage("hello", LevelInfo)
	if id != "" {
		t.Errorf("CaptureMessage without init should return empty string, got %q", id)
	}
}

func TestCaptureExceptionWithRequestWithoutInit(t *testing.T) {
	globalHub = nil
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	id := CaptureExceptionWithRequest(errors.New("test"), req)
	if id != "" {
		t.Errorf("expected empty string, got %q", id)
	}
}

func TestAddBreadcrumbWithoutInit(t *testing.T) {
	globalHub = nil
	// Should not panic
	AddBreadcrumb(&Breadcrumb{Message: "test"})
}

func TestConfigureScopeWithoutInit(t *testing.T) {
	globalHub = nil
	// Should not panic
	ConfigureScope(func(scope *Scope) {
		scope.SetTag("key", "value")
	})
}

func TestFlushWithoutInit(t *testing.T) {
	globalHub = nil
	result := Flush(time.Second)
	if !result {
		t.Error("Flush without init should return true")
	}
}

func TestGetHubReturnsNilWithoutInit(t *testing.T) {
	globalHub = nil
	if GetHub() != nil {
		t.Error("GetHub should return nil before Init")
	}
}

func setupTestHub(t *testing.T) (*Hub, *mockTransport) {
	t.Helper()
	mt := &mockTransport{}
	hub, err := NewHub(ClientOptions{
		DSN:       "https://testkey@localhost:9999/api/ingest",
		Transport: mt,
	})
	if err != nil {
		t.Fatalf("NewHub failed: %v", err)
	}
	return hub, mt
}

func TestCaptureExceptionIntegration(t *testing.T) {
	hub, mt := setupTestHub(t)
	globalHub = hub
	t.Cleanup(func() { globalHub = nil })

	id := CaptureException(errors.New("something broke"))
	if id == "" {
		t.Fatal("expected event ID")
	}
	if len(mt.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(mt.events))
	}
	ev := mt.events[0]
	if ev.Message != "something broke" {
		t.Errorf("message = %q, want %q", ev.Message, "something broke")
	}
	if ev.Level != LevelError {
		t.Errorf("level = %q, want %q", ev.Level, LevelError)
	}
}

func TestCaptureMessageIntegration(t *testing.T) {
	hub, mt := setupTestHub(t)
	globalHub = hub
	t.Cleanup(func() { globalHub = nil })

	id := CaptureMessage("hello world", LevelWarning)
	if id == "" {
		t.Fatal("expected event ID")
	}
	if len(mt.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(mt.events))
	}
	ev := mt.events[0]
	if ev.Message != "hello world" {
		t.Errorf("message = %q", ev.Message)
	}
	if ev.Level != LevelWarning {
		t.Errorf("level = %q, want %q", ev.Level, LevelWarning)
	}
}

func TestConfigureScopeIntegration(t *testing.T) {
	hub, mt := setupTestHub(t)
	globalHub = hub
	t.Cleanup(func() { globalHub = nil })

	ConfigureScope(func(scope *Scope) {
		scope.SetTag("env", "test")
		scope.SetUser(map[string]any{"id": "user-1"})
	})

	CaptureMessage("with scope", LevelInfo)

	if len(mt.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(mt.events))
	}
	ev := mt.events[0]
	if ev.Tags["env"] != "test" {
		t.Errorf("tag env = %q, want %q", ev.Tags["env"], "test")
	}
	if ev.User["id"] != "user-1" {
		t.Errorf("user id = %v", ev.User["id"])
	}
}
