package metrics

import (
	"fmt"
)

type SubmissionErrorType int

const (
	UnrecoverableError SubmissionErrorType = iota
	RecoverableError
)

type SubmissionError struct {
	Err  error
	Type SubmissionErrorType
}

// NewUnrecoverableError returns an error that is deemed unrecoverable and the
// request should be aborted
func NewUnrecoverableError(err error) SubmissionError {
	return SubmissionError{Err: err, Type: UnrecoverableError}
}

// NewRecoverableError returns an error that is deemed recoverable and the
// request may be retried
func NewRecoverableError(err error) SubmissionError {
	return SubmissionError{Err: err, Type: RecoverableError}
}

// Error returns the error string for the SubmissionError
func (s SubmissionError) Error() string {
	switch {
	case s.Type == RecoverableError && s.Err == nil:
		return "a recoverable error occurred"
	case s.Type == RecoverableError:
		return fmt.Sprintf("a recoverable error occurred: %s", s.Err)
	case s.Type == UnrecoverableError && s.Err == nil:
		return "an unrecoverable error occurred"
	case s.Type == UnrecoverableError:
		return fmt.Sprintf("an unrecoverable error occurred: %s", s.Err)
	default:
		return "an error occurred"
	}
}

// Unwrap returns the underlying error
func (s SubmissionError) Unwrap() error {
	return s.Err
}

type wrapped interface {
	Unwrap() error
}

// IsRecoverable returns whether the error is deemed recoverable and may be retried
func IsRecoverable(err error) bool {
	if s, ok := err.(SubmissionError); ok {
		return s.Type == RecoverableError
	}

	if e, ok := err.(wrapped); ok {
		return IsRecoverable(e.Unwrap())
	}

	return false
}

// IsUnrecoverable returns whether the error is deemed unrecoverable and the request
// should be aborted
func IsUnrecoverable(err error) bool {
	if s, ok := err.(SubmissionError); ok {
		return s.Type == UnrecoverableError
	}

	if e, ok := err.(wrapped); ok {
		return IsUnrecoverable(e.Unwrap())
	}

	return false
}
