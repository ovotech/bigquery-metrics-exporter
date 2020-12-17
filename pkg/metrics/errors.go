package metrics

import (
	"fmt"
)

type submissionErrorType int

const (
	unrecoverableError submissionErrorType = iota
	recoverableError
)

// SubmissionError is returned when an error is encountered publishing metrics
type SubmissionError struct {
	err     error
	errType submissionErrorType
}

// NewUnrecoverableError returns an error that is deemed unrecoverable and the
// request should be aborted
func NewUnrecoverableError(err error) SubmissionError {
	return SubmissionError{err: err, errType: unrecoverableError}
}

// NewRecoverableError returns an error that is deemed recoverable and the
// request may be retried
func NewRecoverableError(err error) SubmissionError {
	return SubmissionError{err: err, errType: recoverableError}
}

// Error returns the error string for the SubmissionError
func (s SubmissionError) Error() string {
	switch {
	case s.errType == recoverableError && s.err == nil:
		return "a recoverable error occurred"
	case s.errType == recoverableError:
		return fmt.Sprintf("a recoverable error occurred: %s", s.err)
	case s.errType == unrecoverableError && s.err == nil:
		return "an unrecoverable error occurred"
	case s.errType == unrecoverableError:
		return fmt.Sprintf("an unrecoverable error occurred: %s", s.err)
	default:
		return "an error occurred"
	}
}

// Unwrap returns the underlying error
func (s SubmissionError) Unwrap() error {
	return s.err
}

type wrapped interface {
	Unwrap() error
}

// IsRecoverable returns whether the error is deemed recoverable and may be retried
func IsRecoverable(err error) bool {
	if s, ok := err.(SubmissionError); ok {
		return s.errType == recoverableError
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
		return s.errType == unrecoverableError
	}

	if e, ok := err.(wrapped); ok {
		return IsUnrecoverable(e.Unwrap())
	}

	return false
}
