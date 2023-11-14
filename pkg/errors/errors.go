package errors

import (
	"errors"
)

var (
	ErrClosed           = errors.New("channel closed")
	ErrCloseWaitTimeOut = errors.New("CLOSE_WAIT timeout")
	ErrFreeByGC         = errors.New("memory freed by garbage collector")
	ErrSessionAging     = errors.New("session aging")
	ErrAuth             = errors.New("authentication failed")
	ErrAbnormalPacket   = errors.New("abnormal packet")
)
