package overflow

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// SpanPayload represents a child span in a transaction envelope.
type SpanPayload struct {
	SpanID       string            `json:"span_id"`
	ParentSpanID string            `json:"parent_span_id,omitempty"`
	Op           string            `json:"op"`
	Description  string            `json:"description,omitempty"`
	Status       string            `json:"status"`
	StartOffset  float64           `json:"start_offset_ms"`
	DurationMs   float64           `json:"duration_ms"`
	Tags         map[string]string `json:"tags,omitempty"`
	Data         map[string]any    `json:"data,omitempty"`
}

// TransactionEnvelope is the payload sent to the ingest endpoint for performance data.
type TransactionEnvelope struct {
	Type           string            `json:"type"`
	TraceID        string            `json:"trace_id"`
	SpanID         string            `json:"span_id"`
	ParentSpanID   string            `json:"parent_span_id,omitempty"`
	Name           string            `json:"transaction"`
	Op             string            `json:"op"`
	Description    string            `json:"description,omitempty"`
	Status         string            `json:"status"`
	HTTPMethod     string            `json:"http_method,omitempty"`
	HTTPStatusCode int               `json:"http_status_code,omitempty"`
	StartTimestamp string            `json:"start_timestamp"`
	EndTimestamp   string            `json:"timestamp"`
	Environment    string            `json:"environment,omitempty"`
	Release        string            `json:"release,omitempty"`
	Platform       string            `json:"platform,omitempty"`
	Tags           map[string]string `json:"tags,omitempty"`
	Data           map[string]any    `json:"data,omitempty"`
	SDK            SDKInfo           `json:"sdk"`
	Spans          []SpanPayload     `json:"spans"`
}

func generateID(byteLen int) string {
	b := make([]byte, byteLen)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// Span represents a child span within a transaction.
type Span struct {
	spanID      string
	parentID    string
	op          string
	description string
	status      string
	startTime   time.Time
	endTime     time.Time
	tags        map[string]string
	data        map[string]any
}

// SetTag sets a tag on the span.
func (s *Span) SetTag(key, value string) {
	s.tags[key] = value
}

// SetData sets data on the span.
func (s *Span) SetData(key string, value any) {
	s.data[key] = value
}

// SetStatus sets the span status.
func (s *Span) SetStatus(status string) {
	s.status = status
}

// Finish completes the span.
func (s *Span) Finish() {
	s.endTime = time.Now()
}

func (s *Span) durationMs() float64 {
	return float64(s.endTime.Sub(s.startTime).Microseconds()) / 1000.0
}

func (s *Span) offsetMs(txnStart time.Time) float64 {
	return float64(s.startTime.Sub(txnStart).Microseconds()) / 1000.0
}

type txnCtxKey struct{}

// Transaction represents a top-level performance operation.
type Transaction struct {
	traceID     string
	spanID      string
	name        string
	op          string
	description string
	status      string
	httpMethod  string
	httpStatus  int
	startTime   time.Time
	tags        map[string]string
	data        map[string]any

	mu    sync.Mutex
	spans []*Span

	hub *Hub
}

// StartChild creates a child span under this transaction.
func (t *Transaction) StartChild(op, description string) *Span {
	s := &Span{
		spanID:      generateID(8),
		parentID:    t.spanID,
		op:          op,
		description: description,
		status:      "ok",
		startTime:   time.Now(),
		tags:        make(map[string]string),
		data:        make(map[string]any),
	}
	t.mu.Lock()
	t.spans = append(t.spans, s)
	t.mu.Unlock()
	return s
}

// SetTag sets a tag on the transaction.
func (t *Transaction) SetTag(key, value string) {
	t.tags[key] = value
}

// SetData sets data on the transaction.
func (t *Transaction) SetData(key string, value any) {
	t.data[key] = value
}

// SetHTTPStatus sets the HTTP status and auto-sets error status for 5xx.
func (t *Transaction) SetHTTPStatus(code int) {
	t.httpStatus = code
	if code >= 500 {
		t.status = "error"
	}
}

// SetStatus sets the transaction status.
func (t *Transaction) SetStatus(status string) {
	t.status = status
}

// Finish completes the transaction and sends it.
func (t *Transaction) Finish() {
	endTime := time.Now()

	t.mu.Lock()
	spans := make([]*Span, len(t.spans))
	copy(spans, t.spans)
	t.mu.Unlock()

	// Auto-finish unfinished spans
	for _, s := range spans {
		if s.endTime.IsZero() {
			s.endTime = endTime
		}
	}

	spanPayloads := make([]SpanPayload, 0, len(spans))
	for _, s := range spans {
		sp := SpanPayload{
			SpanID:       s.spanID,
			ParentSpanID: s.parentID,
			Op:           s.op,
			Description:  s.description,
			Status:       s.status,
			StartOffset:  s.offsetMs(t.startTime),
			DurationMs:   s.durationMs(),
		}
		if len(s.tags) > 0 {
			sp.Tags = s.tags
		}
		if len(s.data) > 0 {
			sp.Data = s.data
		}
		spanPayloads = append(spanPayloads, sp)
	}

	env := TransactionEnvelope{
		Type:           "transaction",
		TraceID:        t.traceID,
		SpanID:         t.spanID,
		Name:           t.name,
		Op:             t.op,
		Description:    t.description,
		Status:         t.status,
		HTTPMethod:     t.httpMethod,
		HTTPStatusCode: t.httpStatus,
		StartTimestamp:  t.startTime.UTC().Format(time.RFC3339Nano),
		EndTimestamp:    endTime.UTC().Format(time.RFC3339Nano),
		Platform:       "go",
		SDK: SDKInfo{
			Name:    "overflow-go",
			Version: Version,
		},
		Spans: spanPayloads,
	}

	if t.hub != nil {
		env.Environment = t.hub.client.options.Environment
		env.Release = t.hub.client.options.Release
	}
	if len(t.tags) > 0 {
		env.Tags = t.tags
	}
	if len(t.data) > 0 {
		env.Data = t.data
	}

	if t.hub != nil && t.hub.client != nil {
		if t.hub.client.options.Debug {
			fmt.Printf("[overflow] sending transaction %s %s\n", t.traceID, t.name)
		}
		body, err := json.Marshal(env)
		if err != nil {
			return
		}
		if ht, ok := t.hub.client.transport.(*HTTPTransport); ok {
			go ht.SendRaw(body)
		}
	}
}

// StartTransaction creates a new transaction. Call Finish() to send it.
func StartTransaction(ctx context.Context, name, op string) (context.Context, *Transaction) {
	hub := GetHub()
	txn := &Transaction{
		traceID:   generateID(16),
		spanID:    generateID(8),
		name:      name,
		op:        op,
		status:    "ok",
		startTime: time.Now(),
		tags:      make(map[string]string),
		data:      make(map[string]any),
		hub:       hub,
	}
	return context.WithValue(ctx, txnCtxKey{}, txn), txn
}

// TransactionFromContext retrieves the active transaction from context.
func TransactionFromContext(ctx context.Context) *Transaction {
	if txn, ok := ctx.Value(txnCtxKey{}).(*Transaction); ok {
		return txn
	}
	return nil
}
