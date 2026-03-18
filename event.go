package overflow

import (
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Level represents the severity of an event.
type Level string

const (
	LevelDebug   Level = "debug"
	LevelInfo    Level = "info"
	LevelWarning Level = "warning"
	LevelError   Level = "error"
	LevelFatal   Level = "fatal"
)

// Event represents an error or message event to be sent to Overflow.
type Event struct {
	EventID     string            `json:"event_id,omitempty"`
	Message     string            `json:"message,omitempty"`
	Level       Level             `json:"level,omitempty"`
	Platform    string            `json:"platform,omitempty"`
	Timestamp   string            `json:"timestamp,omitempty"`
	Fingerprint []string          `json:"fingerprint,omitempty"`
	Exception   *ExceptionData    `json:"exception,omitempty"`
	Contexts    map[string]any    `json:"contexts,omitempty"`
	Tags        map[string]string `json:"tags,omitempty"`
	Breadcrumbs []Breadcrumb      `json:"breadcrumbs,omitempty"`
	Request     map[string]any    `json:"request,omitempty"`
	User        map[string]any    `json:"user,omitempty"`
	SDK         SDKInfo           `json:"sdk,omitempty"`
	Environment string            `json:"environment,omitempty"`
	Release     string            `json:"release,omitempty"`
	ServerName  string            `json:"server_name,omitempty"`
}

// ExceptionData holds structured exception information.
type ExceptionData struct {
	Values []ExceptionValue `json:"values"`
}

// ExceptionValue represents a single exception in the chain.
type ExceptionValue struct {
	Type       string      `json:"type"`
	Value      string      `json:"value"`
	Stacktrace *Stacktrace `json:"stacktrace,omitempty"`
}

// Stacktrace holds a list of stack frames.
type Stacktrace struct {
	Frames []Frame `json:"frames"`
}

// Frame represents a single stack frame.
type Frame struct {
	Module   string `json:"module,omitempty"`
	Function string `json:"function,omitempty"`
	Filename string `json:"filename,omitempty"`
	Lineno   int    `json:"lineno,omitempty"`
	Colno    int    `json:"colno,omitempty"`
	AbsPath  string `json:"abs_path,omitempty"`
}

// Breadcrumb records an action or event that occurred before an error.
type Breadcrumb struct {
	Type      string         `json:"type,omitempty"`
	Category  string         `json:"category,omitempty"`
	Message   string         `json:"message,omitempty"`
	Data      map[string]any `json:"data,omitempty"`
	Level     string         `json:"level,omitempty"`
	Timestamp time.Time      `json:"timestamp,omitempty"`
}

// SDKInfo identifies the SDK sending the event.
type SDKInfo struct {
	Name    string `json:"name,omitempty"`
	Version string `json:"version,omitempty"`
}

// newEventID generates a new UUID for an event.
func newEventID() string {
	return uuid.New().String()
}

// newEvent creates a base event with defaults populated.
func newEvent() *Event {
	return &Event{
		EventID:  newEventID(),
		Platform: "go",
		SDK: SDKInfo{
			Name:    "overflow-go",
			Version: Version,
		},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
}

// extractException creates structured exception data from a Go error,
// including a stack trace captured from the calling goroutine.
func extractException(err error) *ExceptionData {
	frames := captureStacktrace(3) // skip extractException, CaptureException, public API

	typeName := fmt.Sprintf("%T", err)
	// Clean up common Go error type prefixes
	if strings.HasPrefix(typeName, "*") {
		typeName = typeName[1:]
	}

	return &ExceptionData{
		Values: []ExceptionValue{
			{
				Type:  typeName,
				Value: err.Error(),
				Stacktrace: &Stacktrace{
					Frames: frames,
				},
			},
		},
	}
}

// captureStacktrace captures the current goroutine's stack frames.
func captureStacktrace(skip int) []Frame {
	pcs := make([]uintptr, 50)
	n := runtime.Callers(skip+1, pcs)
	if n == 0 {
		return nil
	}
	pcs = pcs[:n]
	runtimeFrames := runtime.CallersFrames(pcs)

	var frames []Frame
	for {
		f, more := runtimeFrames.Next()
		if f.Function == "" {
			if !more {
				break
			}
			continue
		}

		module := f.Function
		function := f.Function
		if idx := strings.LastIndex(f.Function, "."); idx != -1 {
			module = f.Function[:idx]
			function = f.Function[idx+1:]
		}

		frames = append(frames, Frame{
			Module:   module,
			Function: function,
			Filename: f.File,
			Lineno:   f.Line,
			AbsPath:  f.File,
		})

		if !more {
			break
		}
	}

	// oldest frame first
	for i, j := 0, len(frames)-1; i < j; i, j = i+1, j-1 {
		frames[i], frames[j] = frames[j], frames[i]
	}
	return frames
}
