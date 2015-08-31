package main

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"sync"
	"time"
)

type _escLocalFS struct{}

var _escLocal _escLocalFS

type _escStaticFS struct{}

var _escStatic _escStaticFS

type _escDir struct {
	fs   http.FileSystem
	name string
}

type _escFile struct {
	compressed string
	size       int64
	local      string
	isDir      bool

	data []byte
	once sync.Once
	name string
}

func (_escLocalFS) Open(name string) (http.File, error) {
	f, present := _escData[path.Clean(name)]
	if !present {
		return nil, os.ErrNotExist
	}
	return os.Open(f.local)
}

func (_escStaticFS) prepare(name string) (*_escFile, error) {
	f, present := _escData[path.Clean(name)]
	if !present {
		return nil, os.ErrNotExist
	}
	var err error
	f.once.Do(func() {
		f.name = path.Base(name)
		if f.size == 0 {
			return
		}
		var gr *gzip.Reader
		b64 := base64.NewDecoder(base64.StdEncoding, bytes.NewBufferString(f.compressed))
		gr, err = gzip.NewReader(b64)
		if err != nil {
			return
		}
		f.data, err = ioutil.ReadAll(gr)
	})
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (fs _escStaticFS) Open(name string) (http.File, error) {
	f, err := fs.prepare(name)
	if err != nil {
		return nil, err
	}
	return f.File()
}

func (dir _escDir) Open(name string) (http.File, error) {
	return dir.fs.Open(dir.name + name)
}

func (f *_escFile) File() (http.File, error) {
	type httpFile struct {
		*bytes.Reader
		*_escFile
	}
	return &httpFile{
		Reader:   bytes.NewReader(f.data),
		_escFile: f,
	}, nil
}

func (f *_escFile) Close() error {
	return nil
}

func (f *_escFile) Readdir(count int) ([]os.FileInfo, error) {
	return nil, nil
}

func (f *_escFile) Stat() (os.FileInfo, error) {
	return f, nil
}

func (f *_escFile) Name() string {
	return f.name
}

func (f *_escFile) Size() int64 {
	return f.size
}

func (f *_escFile) Mode() os.FileMode {
	return 0
}

func (f *_escFile) ModTime() time.Time {
	return time.Time{}
}

func (f *_escFile) IsDir() bool {
	return f.isDir
}

func (f *_escFile) Sys() interface{} {
	return f
}

// FS returns a http.Filesystem for the embedded assets. If useLocal is true,
// the filesystem's contents are instead used.
func FS(useLocal bool) http.FileSystem {
	if useLocal {
		return _escLocal
	}
	return _escStatic
}

// Dir returns a http.Filesystem for the embedded assets on a given prefix dir.
// If useLocal is true, the filesystem's contents are instead used.
func Dir(useLocal bool, name string) http.FileSystem {
	if useLocal {
		return _escDir{fs: _escLocal, name: name}
	}
	return _escDir{fs: _escStatic, name: name}
}

// FSByte returns the named file from the embedded assets. If useLocal is
// true, the filesystem's contents are instead used.
func FSByte(useLocal bool, name string) ([]byte, error) {
	if useLocal {
		f, err := _escLocal.Open(name)
		if err != nil {
			return nil, err
		}
		return ioutil.ReadAll(f)
	}
	f, err := _escStatic.prepare(name)
	if err != nil {
		return nil, err
	}
	return f.data, nil
}

// FSMustByte is the same as FSByte, but panics if name is not present.
func FSMustByte(useLocal bool, name string) []byte {
	b, err := FSByte(useLocal, name)
	if err != nil {
		panic(err)
	}
	return b
}

// FSString is the string version of FSByte.
func FSString(useLocal bool, name string) (string, error) {
	b, err := FSByte(useLocal, name)
	return string(b), err
}

// FSMustString is the string version of FSMustByte.
func FSMustString(useLocal bool, name string) string {
	return string(FSMustByte(useLocal, name))
}

var _escData = map[string]*_escFile{

	"/index.html": {
		local: "static/index.html",
		size:  5487,
		compressed: `
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
ExdHwG8VAAA=
`,
	},

	"/": {
		isDir: true,
		local: "static",
	},
}
