package overflow

import (
	"fmt"
	"math/rand"
	"net/http"
	"time"
)

// statusWriter wraps http.ResponseWriter to capture the status code.
type statusWriter struct {
	http.ResponseWriter
	code int
}

func (w *statusWriter) WriteHeader(code int) {
	w.code = code
	w.ResponseWriter.WriteHeader(code)
}

// HTTPMiddleware returns an http.Handler middleware that captures panics
// and reports them to Overflow. When TracesSampleRate > 0, it also creates
// a performance transaction for each request.
//
//	mux := http.NewServeMux()
//	mux.HandleFunc("/", handler)
//	http.ListenAndServe(":8080", overflow.HTTPMiddleware()(mux))
func HTTPMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hub := GetHub()
			if hub != nil {
				hub.AddBreadcrumb(&Breadcrumb{
					Type:     "http",
					Category: "request",
					Message:  fmt.Sprintf("%s %s", r.Method, r.URL.Path),
					Data: map[string]any{
						"method": r.Method,
						"url":    r.URL.String(),
					},
					Level:     "info",
					Timestamp: time.Now(),
				})
			}

			// Start a transaction if tracing is enabled
			var txn *Transaction
			ctx := r.Context()
			if hub != nil && hub.client.options.TracesSampleRate > 0 {
				rate := hub.client.options.TracesSampleRate
				if rate >= 1.0 || rand.Float64() < rate {
					name := fmt.Sprintf("%s %s", r.Method, r.URL.Path)
					ctx, txn = StartTransaction(ctx, name, "http.server")
					txn.httpMethod = r.Method
					txn.SetTag("http.method", r.Method)
					txn.SetTag("http.url", r.URL.Path)
				}
			}

			sw := &statusWriter{ResponseWriter: w, code: http.StatusOK}

			defer func() {
				if rv := recover(); rv != nil {
					if hub != nil {
						var err error
						switch v := rv.(type) {
						case error:
							err = v
						default:
							err = fmt.Errorf("%v", v)
						}

						event := newEvent()
						event.Level = LevelFatal
						event.Message = err.Error()
						event.Exception = extractException(err)
						event.Request = map[string]any{
							"method":  r.Method,
							"url":     r.URL.String(),
							"headers": flattenHeaders(r.Header),
						}
						hub.Scope().applyToEvent(event)
						hub.Client().applyOptions(event)
						hub.Client().Send(event)
						hub.Client().Flush(2 * time.Second)
					}

					if txn != nil {
						txn.SetStatus("error")
						txn.SetHTTPStatus(http.StatusInternalServerError)
						txn.Finish()
					}
					// Re-panic so the default recovery behavior runs
					panic(rv)
				}

				// Finish transaction on normal completion
				if txn != nil {
					txn.SetHTTPStatus(sw.code)
					txn.Finish()
				}
			}()

			next.ServeHTTP(sw, r.WithContext(ctx))
		})
	}
}

func flattenHeaders(h http.Header) map[string]string {
	out := make(map[string]string, len(h))
	for k, v := range h {
		if len(v) > 0 {
			out[k] = v[0]
		}
	}
	return out
}
