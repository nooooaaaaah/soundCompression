package flac

import "fmt"

type EncodingError struct {
	Stage string
	Err   error
}

func (e *EncodingError) Error() string {
	return fmt.Sprintf("encoding error at %s: %v", e.Stage, e.Err)
}

func NewEncodingError(stage string, err error) *EncodingError {
	return &EncodingError{Stage: stage, Err: err}
}
