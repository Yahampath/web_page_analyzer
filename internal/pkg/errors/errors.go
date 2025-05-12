package errors

import (
	"errors"
	"fmt"
	"runtime"
)

// New creates a new instance of the base error
func New (msg string) error {
	return fmt.Errorf("%s: %s", msg, filePath())
}

// Wrap creates a new error of the wrapped error
func Wrap (err error, msg string) error {
	return fmt.Errorf("%s %s \ncaused by: %w", msg, filePath(), err)
}

// Is checks if the error is equal to the target
func Is(err error, target error) bool {
	return errors.Is(err, target)
}

// As returns the wrapped error
func As(err error, target interface{}) bool {
	return errors.As(err, target)
}

func Errorf(format string, args ...interface{}) error {
	args = append(args, filePath())
	return fmt.Errorf(format+ ` %s`, args...)
}

func filePath() string {
	pc, f, l, ok := runtime.Caller(2)
	fn := `unknown`
	if ok {
		fn = runtime.FuncForPC(pc).Name()
	}
	return fmt.Sprintf("at %s\n\t%s:%d", fn, f, l)
}