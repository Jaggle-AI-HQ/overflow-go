package overflow

import (
	"net/http"
	"sync"
)

// Scope holds contextual data that is applied to all events.
type Scope struct {
	mu          sync.RWMutex
	tags        map[string]string
	contexts    map[string]any
	user        map[string]any
	fingerprint []string
	breadcrumbs []Breadcrumb
	request     *http.Request
}

// NewScope returns an empty scope.
func NewScope() *Scope {
	return &Scope{
		tags:     make(map[string]string),
		contexts: make(map[string]any),
	}
}

// SetTag sets a tag key/value pair.
func (s *Scope) SetTag(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tags[key] = value
}

// SetContext sets a named context object.
func (s *Scope) SetContext(key string, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.contexts[key] = value
}

// SetUser sets user information on the scope.
func (s *Scope) SetUser(user map[string]any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.user = user
}

// SetRequest associates an HTTP request with the scope. All events captured
// on this scope will include request context (method, URL, headers).
func (s *Scope) SetRequest(r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.request = r
}

// SetFingerprint overrides the automatic fingerprint grouping.
func (s *Scope) SetFingerprint(fingerprint []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fingerprint = fingerprint
}

// AddBreadcrumb adds a breadcrumb, respecting the max limit.
func (s *Scope) AddBreadcrumb(breadcrumb *Breadcrumb, maxBreadcrumbs int) {
	if maxBreadcrumbs <= 0 {
		maxBreadcrumbs = 100
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.breadcrumbs = append(s.breadcrumbs, *breadcrumb)
	if len(s.breadcrumbs) > maxBreadcrumbs {
		s.breadcrumbs = s.breadcrumbs[len(s.breadcrumbs)-maxBreadcrumbs:]
	}
}

// Clear resets the scope to empty.
func (s *Scope) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tags = make(map[string]string)
	s.contexts = make(map[string]any)
	s.user = nil
	s.fingerprint = nil
	s.breadcrumbs = nil
	s.request = nil
}

// ApplyToEvent merges scope data into the event.
func (s *Scope) ApplyToEvent(event *Event) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.tags) > 0 {
		if event.Tags == nil {
			event.Tags = make(map[string]string)
		}
		for k, v := range s.tags {
			if _, exists := event.Tags[k]; !exists {
				event.Tags[k] = v
			}
		}
	}

	if len(s.contexts) > 0 {
		if event.Contexts == nil {
			event.Contexts = make(map[string]any)
		}
		for k, v := range s.contexts {
			if _, exists := event.Contexts[k]; !exists {
				event.Contexts[k] = v
			}
		}
	}

	if s.user != nil && event.User == nil {
		event.User = s.user
	}

	if len(s.fingerprint) > 0 && len(event.Fingerprint) == 0 {
		event.Fingerprint = s.fingerprint
	}

	if len(s.breadcrumbs) > 0 {
		event.Breadcrumbs = append(s.breadcrumbs, event.Breadcrumbs...)
	}

	if s.request != nil && event.Request == nil {
		event.Request = map[string]any{
			"method":  s.request.Method,
			"url":     s.request.URL.String(),
			"headers": FlattenHeaders(s.request.Header),
		}
	}
}
