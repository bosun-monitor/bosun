// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package svc

import (
	"errors"
	"github.com/StackExchange/bosun/_third_party/code.google.com/p/winsvc/winapi"
	"syscall"
)

// event represents auto-reset, initially non-signaled windows event.
// It is used to communicate between go and asm parts of this package.
type event struct {
	h syscall.Handle
}

func newEvent() (*event, error) {
	h, err := winapi.CreateEvent(nil, 0, 0, nil)
	if err != nil {
		return nil, err
	}
	return &event{h: h}, nil
}

func (e *event) Close() error {
	return syscall.CloseHandle(e.h)
}

func (e *event) Set() error {
	return winapi.SetEvent(e.h)
}

func (e *event) Wait() error {
	s, err := syscall.WaitForSingleObject(e.h, syscall.INFINITE)
	switch s {
	case syscall.WAIT_OBJECT_0:
		break
	case syscall.WAIT_FAILED:
		return err
	default:
		return errors.New("unexpected result from WaitForSingleObject")
	}
	return nil
}
