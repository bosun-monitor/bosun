package memdb

import (
	"testing"

	"bosun.org/_third_party/github.com/syndtr/goleveldb/leveldb/testutil"
)

func TestMemDB(t *testing.T) {
	testutil.RunSuite(t, "MemDB Suite")
}
