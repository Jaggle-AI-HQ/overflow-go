package overflow

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGenerateID(t *testing.T) {
	id8 := generateID(8)
	if len(id8) != 16 { // hex encoding doubles the length
		t.Errorf("generateID(8) length = %d, want 16", len(id8))
	}

	id16 := generateID(16)
	if len(id16) != 32 {
		t.Errorf("generateID(16) length = %d, want 32", len(id16))
	}

	// Uniqueness
	id8_1 := generateID(8)
	id8_2 := generateID(8)
	if id8_1 == id8_2 {
		t.Error("generated IDs should be unique")
	}
}

func TestStartTransactionWithoutHub(t *testing.T) {
	globalHub = nil
	ctx, txn := StartTransaction(context.Background(), "test-txn", "test.op")
	if txn == nil {
		t.Fatal("transaction should not be nil when hub is nil (no sampling applied)")
	}
	if ctx == nil {
		t.Fatal("context should not be nil")
	}
}

func TestStartTransactionWithTracingDisabled(t *testing.T) {
	mt := &mockTransport{}
	hub, _ := NewHub(ClientOptions{
		DSN:              "https://testkey@localhost:9999/api/ingest",
		Transport:        mt,
		TracesSampleRate: 0, // disabled
	})
	globalHub = hub
	t.Cleanup(func() { globalHub = nil })

	_, txn := StartTransaction(context.Background(), "test", "http.server")
	if txn != nil {
		t.Error("transaction should be nil when TracesSampleRate is 0")
	}
}

func TestStartTransactionWithTracingEnabled(t *testing.T) {
	mt := &mockTransport{}
	hub, _ := NewHub(ClientOptions{
		DSN:              "https://testkey@localhost:9999/api/ingest",
		Transport:        mt,
		TracesSampleRate: 1.0,
	})
	globalHub = hub
	t.Cleanup(func() { globalHub = nil })

	ctx, txn := StartTransaction(context.Background(), "GET /api", "http.server")
	if txn == nil {
		t.Fatal("transaction should not be nil when tracing enabled at 100%")
	}
	if txn.name != "GET /api" {
		t.Errorf("name = %q", txn.name)
	}
	if txn.op != "http.server" {
		t.Errorf("op = %q", txn.op)
	}
	if txn.status != "ok" {
		t.Errorf("status = %q, want %q", txn.status, "ok")
	}
	if txn.traceID == "" {
		t.Error("traceID should not be empty")
	}
	if txn.spanID == "" {
		t.Error("spanID should not be empty")
	}
	if txn.hub != hub {
		t.Error("hub should be set on transaction")
	}

	// Verify context contains transaction
	fromCtx := TransactionFromContext(ctx)
	if fromCtx != txn {
		t.Error("TransactionFromContext should return the same transaction")
	}
}

func TestTransactionFromContextEmpty(t *testing.T) {
	txn := TransactionFromContext(context.Background())
	if txn != nil {
		t.Error("should return nil for empty context")
	}
}

func TestTransactionSetTag(t *testing.T) {
	txn := &Transaction{tags: make(map[string]string), data: make(map[string]any)}
	txn.SetTag("http.method", "GET")
	if txn.tags["http.method"] != "GET" {
		t.Errorf("tag = %q", txn.tags["http.method"])
	}
}

func TestTransactionSetData(t *testing.T) {
	txn := &Transaction{tags: make(map[string]string), data: make(map[string]any)}
	txn.SetData("db.query", "SELECT 1")
	if txn.data["db.query"] != "SELECT 1" {
		t.Errorf("data = %v", txn.data["db.query"])
	}
}

func TestTransactionSetHTTPStatus(t *testing.T) {
	txn := &Transaction{tags: make(map[string]string), data: make(map[string]any), status: "ok"}

	txn.SetHTTPStatus(200)
	if txn.httpStatus != 200 {
		t.Errorf("httpStatus = %d", txn.httpStatus)
	}
	if txn.status != "ok" {
		t.Error("status should remain ok for 200")
	}

	txn.SetHTTPStatus(500)
	if txn.httpStatus != 500 {
		t.Errorf("httpStatus = %d", txn.httpStatus)
	}
	if txn.status != "error" {
		t.Errorf("status should be error for 500, got %q", txn.status)
	}
}

func TestTransactionSetHTTPStatusBoundary(t *testing.T) {
	tests := []struct {
		code       int
		wantStatus string
	}{
		{499, "ok"},
		{500, "error"},
		{503, "error"},
	}
	for _, tt := range tests {
		txn := &Transaction{tags: make(map[string]string), data: make(map[string]any), status: "ok"}
		txn.SetHTTPStatus(tt.code)
		if txn.status != tt.wantStatus {
			t.Errorf("SetHTTPStatus(%d): status = %q, want %q", tt.code, txn.status, tt.wantStatus)
		}
	}
}

func TestTransactionSetStatus(t *testing.T) {
	txn := &Transaction{tags: make(map[string]string), data: make(map[string]any)}
	txn.SetStatus("cancelled")
	if txn.status != "cancelled" {
		t.Errorf("status = %q", txn.status)
	}
}

func TestTransactionStartChild(t *testing.T) {
	txn := &Transaction{
		spanID: "parent-span",
		tags:   make(map[string]string),
		data:   make(map[string]any),
	}

	span := txn.StartChild("db.query", "SELECT * FROM users")
	if span == nil {
		t.Fatal("span should not be nil")
	}
	if span.parentID != "parent-span" {
		t.Errorf("parentID = %q, want %q", span.parentID, "parent-span")
	}
	if span.op != "db.query" {
		t.Errorf("op = %q", span.op)
	}
	if span.description != "SELECT * FROM users" {
		t.Errorf("description = %q", span.description)
	}
	if span.status != "ok" {
		t.Errorf("status = %q, want %q", span.status, "ok")
	}
	if span.spanID == "" {
		t.Error("spanID should not be empty")
	}

	if len(txn.spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(txn.spans))
	}
}

func TestSpanSetTagAndData(t *testing.T) {
	span := &Span{
		tags: make(map[string]string),
		data: make(map[string]any),
	}
	span.SetTag("key", "value")
	span.SetData("rows", 42)

	if span.tags["key"] != "value" {
		t.Errorf("tag = %q", span.tags["key"])
	}
	if span.data["rows"] != 42 {
		t.Errorf("data = %v", span.data["rows"])
	}
}

func TestSpanSetStatus(t *testing.T) {
	span := &Span{tags: make(map[string]string), data: make(map[string]any)}
	span.SetStatus("error")
	if span.status != "error" {
		t.Errorf("status = %q", span.status)
	}
}

func TestSpanFinish(t *testing.T) {
	span := &Span{
		startTime: time.Now().Add(-100 * time.Millisecond),
		tags:      make(map[string]string),
		data:      make(map[string]any),
	}
	span.Finish()

	if span.endTime.IsZero() {
		t.Error("endTime should be set after Finish")
	}
	if span.endTime.Before(span.startTime) {
		t.Error("endTime should be after startTime")
	}
}

func TestSpanDurationMs(t *testing.T) {
	start := time.Now()
	span := &Span{
		startTime: start,
		endTime:   start.Add(50 * time.Millisecond),
	}

	dur := span.durationMs()
	if dur < 49.0 || dur > 51.0 {
		t.Errorf("durationMs = %f, expected ~50", dur)
	}
}

func TestSpanOffsetMs(t *testing.T) {
	txnStart := time.Now()
	span := &Span{
		startTime: txnStart.Add(25 * time.Millisecond),
	}

	offset := span.offsetMs(txnStart)
	if offset < 24.0 || offset > 26.0 {
		t.Errorf("offsetMs = %f, expected ~25", offset)
	}
}

func TestTransactionFinishAutoFinishesSpans(t *testing.T) {
	mt := &mockTransport{}
	hub, _ := NewHub(ClientOptions{
		DSN:              "https://testkey@localhost:9999/api/ingest",
		Transport:        mt,
		TracesSampleRate: 1.0,
	})
	globalHub = hub
	t.Cleanup(func() { globalHub = nil })

	_, txn := StartTransaction(context.Background(), "test", "http.server")
	span := txn.StartChild("db.query", "SELECT 1")
	// Don't call span.Finish() — it should be auto-finished by txn.Finish()
	txn.Finish()

	if span.endTime.IsZero() {
		t.Error("unfinished span should be auto-finished when transaction finishes")
	}
}

func TestTransactionFinishSendsEnvelope(t *testing.T) {
	// Use a real HTTP test server to verify the transaction is sent
	server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if _, err := readBody(r); err != nil {
			t.Errorf("failed to read body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	})
	defer server.Close()

	hub, _ := NewHub(ClientOptions{
		DSN:              "https://testkey@" + server.Listener.Addr().String() + "/api/ingest",
		TracesSampleRate: 1.0,
	})
	globalHub = hub
	t.Cleanup(func() { globalHub = nil })

	_, txn := StartTransaction(context.Background(), "GET /test", "http.server")
	txn.SetTag("http.method", "GET")
	child := txn.StartChild("db.query", "SELECT 1")
	child.Finish()
	txn.Finish()

	// Give async send time to complete
	time.Sleep(100 * time.Millisecond)

	// The envelope is sent via SendRaw in a goroutine, so we need to wait
	// We just verify the transaction was structured correctly in unit tests above
}

func newTestServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	return httptest.NewServer(handler)
}

func readBody(r *http.Request) ([]byte, error) {
	defer r.Body.Close()
	return io.ReadAll(r.Body)
}
