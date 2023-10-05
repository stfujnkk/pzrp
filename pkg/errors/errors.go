package errors

import (
	"errors"
)

var (
	ErrClosed = errors.New("closed")
	ErrCanceled = errors.New("canceled")
)
