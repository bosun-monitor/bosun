// Copyright 2014 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build windows
// +build go1.3

package svc

import "unsafe"

// from sys_386.s and sys_amd64.s
var servicemain byte // type isn't really byte, it's a funcion to be called by service manager

func getServiceMain(r *uintptr) {
	*r = uintptr(unsafe.Pointer(&servicemain))
}
