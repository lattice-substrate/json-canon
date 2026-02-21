// Package jcserr defines the failure taxonomy for json-canon.
//
// Every error returned by the parser, serializer, or CLI maps to exactly one
// FailureClass, which determines the exit code and enables conformance vectors
// to verify failure classification, not just "did it fail."
package jcserr

import "fmt"

// FailureClass is a stable failure category from FAILURE_TAXONOMY.md.
type FailureClass string

const (
	InvalidUTF8     FailureClass = "INVALID_UTF8"
	InvalidGrammar  FailureClass = "INVALID_GRAMMAR"
	DuplicateKey    FailureClass = "DUPLICATE_KEY"
	LoneSurrogate   FailureClass = "LONE_SURROGATE"
	Noncharacter    FailureClass = "NONCHARACTER"
	NumberOverflow  FailureClass = "NUMBER_OVERFLOW"
	NumberNegZero   FailureClass = "NUMBER_NEGZERO"
	NumberUnderflow FailureClass = "NUMBER_UNDERFLOW"
	BoundExceeded   FailureClass = "BOUND_EXCEEDED"
	NotCanonical    FailureClass = "NOT_CANONICAL"
	CLIUsage        FailureClass = "CLI_USAGE"
	InternalIO      FailureClass = "INTERNAL_IO"
	InternalError   FailureClass = "INTERNAL_ERROR"
)

// ExitCode returns the process exit code for this failure class.
func (fc FailureClass) ExitCode() int {
	switch fc {
	case InternalIO, InternalError:
		return 10
	default:
		return 2
	}
}

// Error is the structured error type for all json-canon failures.
type Error struct {
	Class   FailureClass
	Offset  int
	Message string
	Cause   error
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e.Offset >= 0 {
		return fmt.Sprintf("jcserr: %s at byte %d: %s", e.Class, e.Offset, e.Message)
	}
	return fmt.Sprintf("jcserr: %s: %s", e.Class, e.Message)
}

// Unwrap returns the underlying cause, if any.
func (e *Error) Unwrap() error {
	return e.Cause
}

// New creates a new Error with the given class and message.
func New(class FailureClass, offset int, message string) *Error {
	return &Error{Class: class, Offset: offset, Message: message}
}

// Wrap creates a new Error wrapping an existing error.
func Wrap(class FailureClass, offset int, message string, cause error) *Error {
	return &Error{Class: class, Offset: offset, Message: message, Cause: cause}
}
