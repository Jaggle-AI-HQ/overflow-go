package overflow

import (
	"testing"
	"time"
)

// mockTransport records events for testing.
type mockTransport struct {
	events  []*Event
	flushed bool
}

func (m *mockTransport) Send(event *Event) {
	m.events = append(m.events, event)
}

func (m *mockTransport) Flush(timeout time.Duration) bool {
	m.flushed = true
	return true
}

func TestNewClientNoopWhenDSNEmpty(t *testing.T) {
	client, err := NewClient(ClientOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := client.transport.(*noopTransport); !ok {
		t.Error("expected noopTransport when DSN is empty")
	}
}

func TestNewClientWithValidDSN(t *testing.T) {
	client, err := NewClient(ClientOptions{
		DSN: "https://testkey@localhost:9999/api/ingest",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := client.transport.(*HTTPTransport); !ok {
		t.Error("expected HTTPTransport")
	}
}

func TestNewClientWithInvalidDSN(t *testing.T) {
	_, err := NewClient(ClientOptions{
		DSN: "https://example.com/no-key",
	})
	if err == nil {
		t.Fatal("expected error for DSN without public key")
	}
}

func TestNewClientCustomTransport(t *testing.T) {
	mt := &mockTransport{}
	client, err := NewClient(ClientOptions{
		DSN:       "https://testkey@localhost:9999/api/ingest",
		Transport: mt,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	event := NewEvent()
	event.Message = "test"
	client.Send(event)

	if len(mt.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(mt.events))
	}
}

func TestNewClientDefaultSampleRate(t *testing.T) {
	client, err := NewClient(ClientOptions{
		DSN:       "https://testkey@localhost:9999/api/ingest",
		Transport: &mockTransport{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.options.SampleRate != 1.0 {
		t.Errorf("SampleRate = %f, want 1.0", client.options.SampleRate)
	}
}

func TestNewClientDefaultMaxBreadcrumbs(t *testing.T) {
	client, err := NewClient(ClientOptions{
		DSN:       "https://testkey@localhost:9999/api/ingest",
		Transport: &mockTransport{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.options.MaxBreadcrumbs != 100 {
		t.Errorf("MaxBreadcrumbs = %d, want 100", client.options.MaxBreadcrumbs)
	}
}

func TestClientSendReturnsEventID(t *testing.T) {
	mt := &mockTransport{}
	client, _ := NewClient(ClientOptions{
		DSN:       "https://testkey@localhost:9999/api/ingest",
		Transport: mt,
	})

	event := NewEvent()
	event.Message = "test"
	id := client.Send(event)
	if id == "" {
		t.Error("Send should return an event ID")
	}
	if id != event.EventID {
		t.Errorf("returned ID %q != event ID %q", id, event.EventID)
	}
}

func TestClientSendSampling(t *testing.T) {
	mt := &mockTransport{}
	client, _ := NewClient(ClientOptions{
		DSN:        "https://testkey@localhost:9999/api/ingest",
		Transport:  mt,
		SampleRate: 0.0,
	})
	// SampleRate 0 gets defaulted to 1.0 by NewClient, but let's override after
	client.options.SampleRate = 0.0

	for i := 0; i < 100; i++ {
		event := NewEvent()
		event.Message = "test"
		client.Send(event)
	}

	// With 0% sample rate, all events should be dropped
	if len(mt.events) != 0 {
		t.Errorf("expected 0 events with 0%% sample rate, got %d", len(mt.events))
	}
}

func TestClientSendBeforeSendHook(t *testing.T) {
	mt := &mockTransport{}
	client, _ := NewClient(ClientOptions{
		DSN:       "https://testkey@localhost:9999/api/ingest",
		Transport: mt,
		BeforeSend: func(event *Event) *Event {
			event.Tags = map[string]string{"modified": "true"}
			return event
		},
	})

	event := NewEvent()
	event.Message = "test"
	client.Send(event)

	if len(mt.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(mt.events))
	}
	if mt.events[0].Tags["modified"] != "true" {
		t.Error("BeforeSend hook did not modify the event")
	}
}

func TestClientSendBeforeSendDropsEvent(t *testing.T) {
	mt := &mockTransport{}
	client, _ := NewClient(ClientOptions{
		DSN:       "https://testkey@localhost:9999/api/ingest",
		Transport: mt,
		BeforeSend: func(event *Event) *Event {
			return nil // drop event
		},
	})

	event := NewEvent()
	id := client.Send(event)
	if id != "" {
		t.Errorf("expected empty ID when event is dropped, got %q", id)
	}
	if len(mt.events) != 0 {
		t.Error("event should not have been sent")
	}
}

func TestClientApplyOptions(t *testing.T) {
	client, _ := NewClient(ClientOptions{
		DSN:         "https://testkey@localhost:9999/api/ingest",
		Transport:   &mockTransport{},
		Environment: "production",
		Release:     "v1.0.0",
		ServerName:  "web-01",
	})

	event := NewEvent()
	client.ApplyOptions(event)

	if event.Environment != "production" {
		t.Errorf("Environment = %q, want %q", event.Environment, "production")
	}
	if event.Release != "v1.0.0" {
		t.Errorf("Release = %q, want %q", event.Release, "v1.0.0")
	}
	if event.ServerName != "web-01" {
		t.Errorf("ServerName = %q, want %q", event.ServerName, "web-01")
	}
}

func TestClientApplyOptionsDoesNotOverride(t *testing.T) {
	client, _ := NewClient(ClientOptions{
		DSN:         "https://testkey@localhost:9999/api/ingest",
		Transport:   &mockTransport{},
		Environment: "production",
		Release:     "v1.0.0",
	})

	event := NewEvent()
	event.Environment = "staging"
	event.Release = "v2.0.0"
	client.ApplyOptions(event)

	if event.Environment != "staging" {
		t.Errorf("Environment should not be overridden, got %q", event.Environment)
	}
	if event.Release != "v2.0.0" {
		t.Errorf("Release should not be overridden, got %q", event.Release)
	}
}

func TestClientFlush(t *testing.T) {
	mt := &mockTransport{}
	client, _ := NewClient(ClientOptions{
		DSN:       "https://testkey@localhost:9999/api/ingest",
		Transport: mt,
	})

	result := client.Flush(time.Second)
	if !result {
		t.Error("Flush should return true")
	}
	if !mt.flushed {
		t.Error("transport Flush should have been called")
	}
}

func TestClientOptions(t *testing.T) {
	client, _ := NewClient(ClientOptions{
		DSN:         "https://testkey@localhost:9999/api/ingest",
		Transport:   &mockTransport{},
		Environment: "test",
	})

	opts := client.Options()
	if opts.Environment != "test" {
		t.Errorf("Options().Environment = %q, want %q", opts.Environment, "test")
	}
}
