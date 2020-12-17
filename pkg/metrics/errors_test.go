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
		{"recoverable", args{SubmissionError{Type: RecoverableError}}, true},
		{"unrecoverable", args{SubmissionError{Type: UnrecoverableError}}, false},
		{"wrapped recoverable", args{fmt.Errorf("an error occurred: %w", SubmissionError{Type: RecoverableError})}, true},
		{"wrapped unrecoverable", args{fmt.Errorf("an error occurred: %w", SubmissionError{Type: UnrecoverableError})}, false},
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
		{"unrecoverable", args{SubmissionError{Type: UnrecoverableError}}, true},
		{"recoverable", args{SubmissionError{Type: RecoverableError}}, false},
		{"wrapped unrecoverable", args{fmt.Errorf("an error occurred: %w", SubmissionError{Type: UnrecoverableError})}, true},
		{"wrapped recoverable", args{fmt.Errorf("an error occurred: %w", SubmissionError{Type: RecoverableError})}, false},
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
		{"error", args{ErrGenericTest}, SubmissionError{ErrGenericTest, RecoverableError}},
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
		{"error", args{ErrGenericTest}, SubmissionError{ErrGenericTest, UnrecoverableError}},
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
		Type SubmissionErrorType
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{"unrecoverable", fields{ErrGenericTest, UnrecoverableError}, "an unrecoverable error occurred: an error occurred"},
		{"recoverable", fields{ErrGenericTest, RecoverableError}, "a recoverable error occurred: an error occurred"},
		{"no wrapped error unrecoverable", fields{Type: UnrecoverableError}, "an unrecoverable error occurred"},
		{"no wrapped error recoverable", fields{Type: RecoverableError}, "a recoverable error occurred"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := SubmissionError{
				Err:  tt.fields.Err,
				Type: tt.fields.Type,
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
		Type SubmissionErrorType
	}
	tests := []struct {
		name   string
		fields fields
		want   error
	}{
		{"unrecoverable", fields{ErrGenericTest, UnrecoverableError}, ErrGenericTest},
		{"recoverable", fields{ErrGenericTest, RecoverableError}, ErrGenericTest},
		{"no wrapped error", fields{Type: UnrecoverableError}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := SubmissionError{
				Err:  tt.fields.Err,
				Type: tt.fields.Type,
			}
			if err := s.Unwrap(); err != tt.want {
				t.Errorf("Unwrap() error = %v, want %v", err, tt.want)
			}
		})
	}
}
