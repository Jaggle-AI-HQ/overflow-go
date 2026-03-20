package overflow

import "net/http"

// Hub is the central point for managing the SDK client and scope.
type Hub struct {
	client *Client
	scope  *Scope
}

// NewHub creates a new Hub with the given options.
func NewHub(options ClientOptions) (*Hub, error) {
	client, err := NewClient(options)
	if err != nil {
		return nil, err
	}
	scope := NewScope()
	return &Hub{client: client, scope: scope}, nil
}

// Client returns the hub's client.
func (h *Hub) Client() *Client {
	return h.client
}

// Scope returns the hub's scope.
func (h *Hub) Scope() *Scope {
	return h.scope
}

// CaptureException creates an event from an error, applies the scope, and sends it.
func (h *Hub) CaptureException(err error) string {
	event := NewEvent()
	event.Level = LevelError
	event.Message = err.Error()
	event.Exception = ExtractException(err)
	h.scope.ApplyToEvent(event)
	h.client.ApplyOptions(event)
	return h.client.Send(event)
}

// CaptureExceptionWithRequest creates an event from an error, attaches HTTP request
// context (method, URL, headers), applies the scope, and sends it.
// The level parameter controls the event severity (e.g. LevelWarning for 4xx,
// LevelError for 5xx).
func (h *Hub) CaptureExceptionWithRequest(err error, r *http.Request, level Level) string {
	event := NewEvent()
	event.Level = level
	event.Message = err.Error()
	event.Exception = ExtractException(err)
	event.Request = map[string]any{
		"method":  r.Method,
		"url":     r.URL.String(),
		"headers": FlattenHeaders(r.Header),
	}
	h.scope.ApplyToEvent(event)
	h.client.ApplyOptions(event)
	return h.client.Send(event)
}

// CaptureMessage creates a message event, applies the scope, and sends it.
func (h *Hub) CaptureMessage(msg string, level Level) string {
	event := NewEvent()
	event.Level = level
	event.Message = msg
	h.scope.ApplyToEvent(event)
	h.client.ApplyOptions(event)
	return h.client.Send(event)
}

// AddBreadcrumb adds a breadcrumb to the hub's scope.
func (h *Hub) AddBreadcrumb(breadcrumb *Breadcrumb) {
	h.scope.AddBreadcrumb(breadcrumb, h.client.options.MaxBreadcrumbs)
}
