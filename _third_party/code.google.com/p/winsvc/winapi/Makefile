# Copyright 2012 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

GOOS=windows

# TODO: use $(GOROOT)/src/pkg/syscall/mksyscall_windows.go here instead
zwinapi_windows.go: event.go eventlog.go registry.go security.go service.go syscall.go
	./mksyscall_windows.pl $^ \
		| gofmt \
		> $@
