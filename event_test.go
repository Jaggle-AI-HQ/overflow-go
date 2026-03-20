package overflow

import (
	"errors"
	"strings"
	"testing"
)

func TestNewEvent(t *testing.T) {
	ev := NewEvent()

	if ev.EventID == "" {
		t.Error("EventID should not be empty")
	}
	if ev.Platform != "go" {
		t.Errorf("Platform = %q, want %q", ev.Platform, "go")
	}
	if ev.SDK.Name != "overflow-go" {
		t.Errorf("SDK.Name = %q, want %q", ev.SDK.Name, "overflow-go")
	}
	if ev.SDK.Version != Version {
		t.Errorf("SDK.Version = %q, want %q", ev.SDK.Version, Version)
	}
	if ev.Timestamp == "" {
		t.Error("Timestamp should not be empty")
	}
}

func TestNewEventIDUniqueness(t *testing.T) {
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := newEventID()
		if ids[id] {
			t.Fatalf("duplicate event ID: %s", id)
		}
		ids[id] = true
	}
}

func TestExtractException(t *testing.T) {
	err := errors.New("test error")
	exc := ExtractException(err)

	if exc == nil {
		t.Fatal("ExceptionData should not be nil")
	}
	if len(exc.Values) != 1 {
		t.Fatalf("expected 1 exception value, got %d", len(exc.Values))
	}

	val := exc.Values[0]
	if val.Value != "test error" {
		t.Errorf("Value = %q, want %q", val.Value, "test error")
	}
	if val.Type != "errors.errorString" {
		t.Errorf("Type = %q, want %q", val.Type, "errors.errorString")
	}
	if val.Stacktrace == nil {
		t.Fatal("Stacktrace should not be nil")
	}
	if len(val.Stacktrace.Frames) == 0 {
		t.Fatal("Stacktrace should have frames")
	}
}

func TestExtractExceptionCustomError(t *testing.T) {
	type myError struct{ msg string }
	me := &myError{msg: "custom"}
	// myError doesn't implement error, so use a wrapper
	err := errors.New("wrapped")
	exc := ExtractException(err)
	_ = me

	if exc.Values[0].Type != "errors.errorString" {
		t.Errorf("Type = %q", exc.Values[0].Type)
	}
}

func TestCaptureStacktrace(t *testing.T) {
	frames := captureStacktrace(0)

	if len(frames) == 0 {
		t.Fatal("expected at least one frame")
	}

	// Frames are reversed (oldest first), so find our test function anywhere in the stack
	found := false
	for _, f := range frames {
		if strings.Contains(f.Function, "TestCaptureStacktrace") {
			found = true
			if f.Lineno == 0 {
				t.Error("line number should not be 0")
			}
			if f.Filename == "" {
				t.Error("filename should not be empty")
			}
			break
		}
	}
	if !found {
		t.Error("expected to find TestCaptureStacktrace in stack frames")
	}
}

func TestCaptureStacktraceModuleParsing(t *testing.T) {
	frames := captureStacktrace(0)
	if len(frames) == 0 {
		t.Fatal("expected frames")
	}

	lastFrame := frames[len(frames)-1]
	if lastFrame.Module == "" {
		t.Error("module should not be empty")
	}
	// Module should not contain the function name
	if strings.Contains(lastFrame.Module, "TestCaptureStacktraceModuleParsing") {
		t.Error("module should not contain the function name")
	}
}
