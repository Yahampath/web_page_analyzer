package errors

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
)

const maxStackDepth = 32

type Error struct {
	msg   string
	cause error
	stack []uintptr
}

func (e *Error) Error() string {
	return e.msg
}

func (e *Error) Unwrap() error {
	return e.cause
}

func (e *Error) Format() string {
	var builder strings.Builder
	builder.WriteString(e.msg)
	if e.cause != nil {
		builder.WriteString("\ncaused by: ")
		if cause, ok := e.cause.(*Error); ok {
			builder.WriteString(cause.Format())
		} else {
			builder.WriteString(e.cause.Error())
		}
	}
	builder.WriteString("\n--- stack trace ---\n")
	for _, pc := range e.stack {
		fn := runtime.FuncForPC(pc)
		if fn != nil {
			file, line := fn.FileLine(pc)
			builder.WriteString(fmt.Sprintf("%s\n\t%s:%d\n", fn.Name(), file, line))
		}
	}
	return builder.String()
}

func captureStack() []uintptr {
	var pcs [maxStackDepth]uintptr
	n := runtime.Callers(3, pcs[:])
	return pcs[0:n]
}

func New(msg string) error {
	return &Error{
		msg:   msg,
		stack: captureStack(),
	}
}

func Errorf(format string, args ...interface{}) error {
	return &Error{
		msg:   fmt.Sprintf(format, args...),
		stack: captureStack(),
	}
}

func Wrap(err error, msg string) error {
	return &Error{
		msg:   msg,
		cause: err,
		stack: captureStack(),
	}
}

func Wrapf(err error, format string, args ...interface{}) error {
	return &Error{
		msg:   fmt.Sprintf(format, args...),
		cause: err,
		stack: captureStack(),
	}
}

func Is(err, target error) bool {
	return errors.Is(err, target)
}

func As(err error, target interface{}) bool {
	return errors.As(err, target)
}

func Cause(err error) error {
	for err != nil {
		cause := errors.Unwrap(err)
		if cause == nil {
			return err
		}
		err = cause
	}
	return nil
}