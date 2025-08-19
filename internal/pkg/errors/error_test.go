package errors

import (
	"errors"
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	err := New("test error")
	if err.Error() != "test error" {
		t.Errorf("expected 'test error', got '%s'", err.Error())
	}

	e, ok := err.(*Error)
	if !ok {
		t.Fatal("New should return a *Error type")
	}

	if len(e.stack) == 0 {
		t.Error("expected stack to be captured")
	}
}

func TestErrorf(t *testing.T) {
	err := Errorf("error with id %d", 123)
	expected := "error with id 123"
	if err.Error() != expected {
		t.Errorf("expected '%s', got '%s'", expected, err.Error())
	}

	e, ok := err.(*Error)
	if !ok {
		t.Fatal("Errorf should return a *Error type")
	}

	if len(e.stack) == 0 {
		t.Error("expected stack to be captured")
	}
}

func TestWrap(t *testing.T) {
	cause := errors.New("original error")
	err := Wrap(cause, "wrapped error")

	if err.Error() != "wrapped error" {
		t.Errorf("expected 'wrapped error', got '%s'", err.Error())
	}

	if !Is(err, cause) {
		t.Error("expected wrapped error to match cause")
	}

	e, ok := err.(*Error)
	if !ok {
		t.Fatal("Wrap should return a *Error type")
	}

	if len(e.stack) == 0 {
		t.Error("expected stack to be captured")
	}
}

func TestWrapf(t *testing.T) {
	cause := errors.New("original error")
	err := Wrapf(cause, "wrapped error with id %d", 456)
	expected := "wrapped error with id 456"
	if err.Error() != expected {
		t.Errorf("expected '%s', got '%s'", expected, err.Error())
	}

	if !Is(err, cause) {
		t.Error("expected wrapped error to match cause")
	}
}

func TestFormat(t *testing.T) {
	cause := New("original error")
	err := Wrap(cause, "wrapped error")

	formatted := err.(*Error).Format()

	if !strings.Contains(formatted, "wrapped error") {
		t.Errorf("formatted error should contain wrapped message")
	}

	if !strings.Contains(formatted, "original error") {
		t.Errorf("formatted error should contain original message")
	}

	if !strings.Contains(formatted, "--- stack trace ---") {
		t.Errorf("formatted error should contain stack trace")
	}

	if !strings.Contains(formatted, "testing.tRunner") {
		t.Errorf("stack trace should contain testing context")
	}
}

func TestCause(t *testing.T) {
	err1 := New("error 1")
	err2 := Wrap(err1, "error 2")
	err3 := Wrap(err2, "error 3")

	if Cause(err3) != err1 {
		t.Errorf("expected cause of err3 to be err1")
	}

	if Cause(err1) != err1 {
		t.Errorf("expected cause of err1 to be itself")
	}
}
