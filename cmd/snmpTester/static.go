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

type _escDirectory struct {
	fs   http.FileSystem
	name string
}

type _escFile struct {
	compressed string
	size       int64
	modtime    int64
	local      string
	isDir      bool

	once sync.Once
	data []byte
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

func (dir _escDirectory) Open(name string) (http.File, error) {
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
	return time.Unix(f.modtime, 0)
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
		return _escDirectory{fs: _escLocal, name: name}
	}
	return _escDirectory{fs: _escStatic, name: name}
}

// FSByte returns the named file from the embedded assets. If useLocal is
// true, the filesystem's contents are instead used.
func FSByte(useLocal bool, name string) ([]byte, error) {
	if useLocal {
		f, err := _escLocal.Open(name)
		if err != nil {
			return nil, err
		}
		b, err := ioutil.ReadAll(f)
		f.Close()
		return b, err
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
		local:   "static/index.html",
		size:    5487,
		modtime: 0,
		compressed: `
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
/2aotDOmblnxvwAAAP//ExdHwG8VAAA=
`,
	},

	"/": {
		isDir: true,
		local: "static",
	},
}
