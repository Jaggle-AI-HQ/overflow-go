package overflow

import (
	"net/http"
	"sync"
	"testing"
)

func TestNewScope(t *testing.T) {
	s := NewScope()
	if s == nil {
		t.Fatal("NewScope should not return nil")
	}
}

func TestScopeSetTag(t *testing.T) {
	s := NewScope()
	s.SetTag("key", "value")

	event := &Event{}
	s.applyToEvent(event)

	if event.Tags["key"] != "value" {
		t.Errorf("tag key = %q, want %q", event.Tags["key"], "value")
	}
}

func TestScopeSetContext(t *testing.T) {
	s := NewScope()
	s.SetContext("device", map[string]any{"os": "linux"})

	event := &Event{}
	s.applyToEvent(event)

	ctx, ok := event.Contexts["device"].(map[string]any)
	if !ok {
		t.Fatal("context should be set")
	}
	if ctx["os"] != "linux" {
		t.Errorf("os = %v", ctx["os"])
	}
}

func TestScopeSetUser(t *testing.T) {
	s := NewScope()
	s.SetUser(map[string]any{"id": "42", "email": "test@example.com"})

	event := &Event{}
	s.applyToEvent(event)

	if event.User["id"] != "42" {
		t.Errorf("user id = %v", event.User["id"])
	}
	if event.User["email"] != "test@example.com" {
		t.Errorf("user email = %v", event.User["email"])
	}
}

func TestScopeSetFingerprint(t *testing.T) {
	s := NewScope()
	s.SetFingerprint([]string{"custom-group"})

	event := &Event{}
	s.applyToEvent(event)

	if len(event.Fingerprint) != 1 || event.Fingerprint[0] != "custom-group" {
		t.Errorf("fingerprint = %v", event.Fingerprint)
	}
}

func TestScopeSetRequest(t *testing.T) {
	s := NewScope()
	req, _ := http.NewRequest("GET", "http://example.com/path", nil)
	req.Header.Set("Accept", "text/html")
	s.SetRequest(req)

	event := &Event{}
	s.applyToEvent(event)

	if event.Request == nil {
		t.Fatal("request should not be nil")
	}
	if event.Request["method"] != "GET" {
		t.Errorf("method = %v", event.Request["method"])
	}
	if event.Request["url"] != "http://example.com/path" {
		t.Errorf("url = %v", event.Request["url"])
	}
}

func TestScopeAddBreadcrumb(t *testing.T) {
	s := NewScope()
	s.AddBreadcrumb(&Breadcrumb{Message: "first"}, 100)
	s.AddBreadcrumb(&Breadcrumb{Message: "second"}, 100)

	event := &Event{}
	s.applyToEvent(event)

	if len(event.Breadcrumbs) != 2 {
		t.Fatalf("expected 2 breadcrumbs, got %d", len(event.Breadcrumbs))
	}
	if event.Breadcrumbs[0].Message != "first" {
		t.Errorf("first breadcrumb = %q", event.Breadcrumbs[0].Message)
	}
}

func TestScopeAddBreadcrumbMaxLimit(t *testing.T) {
	s := NewScope()
	for i := 0; i < 5; i++ {
		s.AddBreadcrumb(&Breadcrumb{Message: "old"}, 3)
	}

	event := &Event{}
	s.applyToEvent(event)

	if len(event.Breadcrumbs) != 3 {
		t.Fatalf("expected 3 breadcrumbs (max), got %d", len(event.Breadcrumbs))
	}
}

func TestScopeAddBreadcrumbDefaultMax(t *testing.T) {
	s := NewScope()
	// maxBreadcrumbs <= 0 defaults to 100
	for i := 0; i < 105; i++ {
		s.AddBreadcrumb(&Breadcrumb{Message: "crumb"}, 0)
	}

	event := &Event{}
	s.applyToEvent(event)

	if len(event.Breadcrumbs) != 100 {
		t.Fatalf("expected 100 breadcrumbs (default max), got %d", len(event.Breadcrumbs))
	}
}

func TestScopeClear(t *testing.T) {
	s := NewScope()
	s.SetTag("key", "value")
	s.SetContext("ctx", "data")
	s.SetUser(map[string]any{"id": "1"})
	s.SetFingerprint([]string{"fp"})
	s.AddBreadcrumb(&Breadcrumb{Message: "bc"}, 100)
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	s.SetRequest(req)

	s.Clear()

	event := &Event{}
	s.applyToEvent(event)

	if len(event.Tags) != 0 {
		t.Error("tags should be empty after clear")
	}
	if len(event.Contexts) != 0 {
		t.Error("contexts should be empty after clear")
	}
	if event.User != nil {
		t.Error("user should be nil after clear")
	}
	if len(event.Fingerprint) != 0 {
		t.Error("fingerprint should be empty after clear")
	}
	if len(event.Breadcrumbs) != 0 {
		t.Error("breadcrumbs should be empty after clear")
	}
	if event.Request != nil {
		t.Error("request should be nil after clear")
	}
}

func TestScopeDoesNotOverrideExistingEventData(t *testing.T) {
	s := NewScope()
	s.SetTag("key", "scope-value")
	s.SetContext("ctx", "scope-data")
	s.SetUser(map[string]any{"id": "scope-user"})
	s.SetFingerprint([]string{"scope-fp"})

	event := &Event{
		Tags:        map[string]string{"key": "event-value"},
		Contexts:    map[string]any{"ctx": "event-data"},
		User:        map[string]any{"id": "event-user"},
		Fingerprint: []string{"event-fp"},
	}
	s.applyToEvent(event)

	if event.Tags["key"] != "event-value" {
		t.Error("scope should not override existing event tags")
	}
	if event.Contexts["ctx"] != "event-data" {
		t.Error("scope should not override existing event contexts")
	}
	if event.User["id"] != "event-user" {
		t.Error("scope should not override existing event user")
	}
	if event.Fingerprint[0] != "event-fp" {
		t.Error("scope should not override existing event fingerprint")
	}
}

func TestScopeBreadcrumbsPrependedToEvent(t *testing.T) {
	s := NewScope()
	s.AddBreadcrumb(&Breadcrumb{Message: "scope-bc"}, 100)

	event := &Event{
		Breadcrumbs: []Breadcrumb{{Message: "event-bc"}},
	}
	s.applyToEvent(event)

	if len(event.Breadcrumbs) != 2 {
		t.Fatalf("expected 2 breadcrumbs, got %d", len(event.Breadcrumbs))
	}
	if event.Breadcrumbs[0].Message != "scope-bc" {
		t.Error("scope breadcrumbs should be prepended")
	}
	if event.Breadcrumbs[1].Message != "event-bc" {
		t.Error("event breadcrumbs should come after scope breadcrumbs")
	}
}

func TestScopeConcurrentAccess(t *testing.T) {
	s := NewScope()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			s.SetTag("key", "value")
			s.AddBreadcrumb(&Breadcrumb{Message: "bc"}, 100)
			event := &Event{}
			s.applyToEvent(event)
		}(i)
	}

	wg.Wait()
}
