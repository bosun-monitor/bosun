#Go OLE

[![GoDoc](https://godoc.org/github.com/go-ole/go-ole?status.svg)](https://godoc.org/github.com/go-ole/go-ole)

Go bindings for Windows COM using shared libraries instead of cgo.

By Yasuhiro Matsumoto.

##INSTALL

To experiment with go-ole, you can just compile and run the example program:

```
go get github.com/go-ole/go-ole
cd /path/to/go-ole/
go test

cd /path/to/go-ole/example/excel
go run excel.go
```

## Continuous Integration

Continuous integration configuration has been added for both Travis-CI and AppVeyor. You will have to add these to your own account for your fork in order for it to run.

**Travis-CI**

Travis-CI was added to check builds on Linux to ensure that `go get` works when cross building. Currently, Travis-CI is not used to test cross-building, but this may be changed in the future. It is also not currently possible to test the library on Linux, since COM API is specific to Windows and it is not currently possible to run a COM server on Linux or even connect to a remote COM server.

**AppVeyor**

AppVeyor is used to build on Windows using the (in-development) test COM server. It is currently only used to test the build and ensure that the code works on Windows. It will be used to register a COM server and then run the test cases based on the test COM server.

The tests currently do run and do pass and this should be maintained with commits.

##LICENSE

Under the MIT License: http://mattn.mit-license.org/2013
