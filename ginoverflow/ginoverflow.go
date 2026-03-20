// Package ginoverflow provides Overflow SDK middleware for the Gin web framework.
//
//	r := gin.Default()
//	r.Use(ginoverflow.Middleware())
package ginoverflow

import (
	"fmt"
	"net/http"
	"time"

	"github.com/Jaggle-AI-HQ/overflow-go"
	"github.com/gin-gonic/gin"
)

// Middleware returns a Gin middleware that captures panics, adds HTTP breadcrumbs,
// and creates performance transactions (when tracing is enabled).
func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		hub := overflow.GetHub()

		// Add HTTP breadcrumb
		if hub != nil {
			hub.AddBreadcrumb(&overflow.Breadcrumb{
				Type:     "http",
				Category: "request",
				Message:  fmt.Sprintf("%s %s", c.Request.Method, c.Request.URL.Path),
				Data: map[string]any{
					"method": c.Request.Method,
					"url":    c.Request.URL.String(),
				},
				Level:     "info",
				Timestamp: time.Now(),
			})

			// Attach request to scope so all events get request context
			hub.Scope().SetRequest(c.Request)
		}

		// Start a transaction (sampling is handled internally)
		ctx, txn := overflow.StartTransaction(c.Request.Context(), fmt.Sprintf("%s %s", c.Request.Method, c.Request.URL.Path), "http.server")
		if txn != nil {
			c.Request = c.Request.WithContext(ctx)
			txn.SetTag("http.method", c.Request.Method)
			txn.SetTag("http.url", c.Request.URL.Path)
		}

		defer func() {
			if rv := recover(); rv != nil {
				// Capture panic as exception with request context
				if hub != nil {
					var err error
					switch v := rv.(type) {
					case error:
						err = v
					default:
						err = fmt.Errorf("%v", v)
					}
					hub.CaptureExceptionWithRequest(err, c.Request)
					hub.Client().Flush(2 * time.Second)
				}

				if txn != nil {
					txn.SetStatus("error")
					txn.SetHTTPStatus(http.StatusInternalServerError)
					txn.Finish()
				}

				// Re-panic so Gin's recovery middleware can handle the response
				panic(rv)
			}

			// Finish transaction on normal completion
			if txn != nil {
				txn.SetHTTPStatus(c.Writer.Status())
				txn.Finish()
			}
		}()

		c.Next()
	}
}
