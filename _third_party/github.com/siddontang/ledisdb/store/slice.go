package store

import (
	"bosun.org/_third_party/github.com/siddontang/ledisdb/store/driver"
)

type Slice interface {
	driver.ISlice
}
