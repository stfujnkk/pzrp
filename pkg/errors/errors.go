package errors

import (
	"errors"
)

var (
	ErrClosed    = errors.New("channel closed")
	ErrCloseWaitTimeOut  = errors.New("CLOSE_WAIT timeout")
	ErrFreeByGC = errors.New("memory freed by garbage collector")
)
