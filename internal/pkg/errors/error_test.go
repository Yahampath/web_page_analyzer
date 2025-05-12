package errors

import (
	"errors"
	"regexp"
	"testing"
)

func TestNewError(t *testing.T) {
	e := errors.New("sample error message")
	if e == nil {
		t.Errorf("expected non-nil error but got nil")
		return
	}

	errString := e.Error()
	match, err := regexp.Match(`sample error message`, []byte(errString))
	if err != nil {
		t.Error(err)
		t.Fail()
		return
	}

	if !match {
		t.Errorf("expected %q to match %q", errString, `sample error message`)
		t.Fail()
		return
	}
}

func TestNewEmbeddedError(t *testing.T) {
	errOne := New("sample error message one")
	errTwo := Wrap(errOne, "sample error message two")

	er := errors.Unwrap(errTwo)
	if er != errOne {
		t.Errorf("expected %v to be equal to %v", er, errOne)
		t.Fail()
		return
	}
}


func TestFilePath(t *testing.T) {
	path := filePath()

	if path == "" {
		t.Fatalf("expected non-empty string but got empty string")
	}

	pattern := `^at testing.tRunner.*`
	match, err := regexp.Match(pattern, []byte(path))
	if err != nil {
		t.Error(err)
		t.Fail()
	}

	if !match {
		t.Fatalf("expected %q to match %q", path, pattern)
	}
}
