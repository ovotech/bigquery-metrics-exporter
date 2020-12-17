package metrics

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
)

var ErrGenericTest = errors.New("an error occurred")

func TestIsRecoverable(t *testing.T) {
	type args struct {
		err error
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{"recoverable", args{SubmissionError{errType: recoverableError}}, true},
		{"unrecoverable", args{SubmissionError{errType: unrecoverableError}}, false},
		{"wrapped recoverable", args{fmt.Errorf("an error occurred: %w", SubmissionError{errType: recoverableError})}, true},
		{"wrapped unrecoverable", args{fmt.Errorf("an error occurred: %w", SubmissionError{errType: unrecoverableError})}, false},
		{"not submission error", args{ErrGenericTest}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsRecoverable(tt.args.err); got != tt.want {
				t.Errorf("IsRecoverable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsUnrecoverable(t *testing.T) {
	type args struct {
		err error
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{"unrecoverable", args{SubmissionError{errType: unrecoverableError}}, true},
		{"recoverable", args{SubmissionError{errType: recoverableError}}, false},
		{"wrapped unrecoverable", args{fmt.Errorf("an error occurred: %w", SubmissionError{errType: unrecoverableError})}, true},
		{"wrapped recoverable", args{fmt.Errorf("an error occurred: %w", SubmissionError{errType: recoverableError})}, false},
		{"not submission error", args{ErrGenericTest}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsUnrecoverable(tt.args.err); got != tt.want {
				t.Errorf("IsUnrecoverable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewRecoverableError(t *testing.T) {
	type args struct {
		err error
	}
	tests := []struct {
		name string
		args args
		want SubmissionError
	}{
		{"error", args{ErrGenericTest}, SubmissionError{ErrGenericTest, recoverableError}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewRecoverableError(tt.args.err); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewRecoverableError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewUnrecoverableError(t *testing.T) {
	type args struct {
		err error
	}
	tests := []struct {
		name string
		args args
		want SubmissionError
	}{
		{"error", args{ErrGenericTest}, SubmissionError{ErrGenericTest, unrecoverableError}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewUnrecoverableError(tt.args.err); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewUnrecoverableError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSubmissionError_Error(t *testing.T) {
	type fields struct {
		Err  error
		Type submissionErrorType
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{"unrecoverable", fields{ErrGenericTest, unrecoverableError}, "an unrecoverable error occurred: an error occurred"},
		{"recoverable", fields{ErrGenericTest, recoverableError}, "a recoverable error occurred: an error occurred"},
		{"no wrapped error unrecoverable", fields{Type: unrecoverableError}, "an unrecoverable error occurred"},
		{"no wrapped error recoverable", fields{Type: recoverableError}, "a recoverable error occurred"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := SubmissionError{
				err:     tt.fields.Err,
				errType: tt.fields.Type,
			}
			if got := s.Error(); got != tt.want {
				t.Errorf("Error() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSubmissionError_Unwrap(t *testing.T) {
	type fields struct {
		Err  error
		Type submissionErrorType
	}
	tests := []struct {
		name   string
		fields fields
		want   error
	}{
		{"unrecoverable", fields{ErrGenericTest, unrecoverableError}, ErrGenericTest},
		{"recoverable", fields{ErrGenericTest, recoverableError}, ErrGenericTest},
		{"no wrapped error", fields{Type: unrecoverableError}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := SubmissionError{
				err:     tt.fields.Err,
				errType: tt.fields.Type,
			}
			if err := s.Unwrap(); err != tt.want {
				t.Errorf("Unwrap() error = %v, want %v", err, tt.want)
			}
		})
	}
}
