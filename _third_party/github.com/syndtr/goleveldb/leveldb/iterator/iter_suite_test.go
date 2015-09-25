package iterator_test

import (
	"testing"

	"bosun.org/_third_party/github.com/syndtr/goleveldb/leveldb/testutil"
)

func TestIterator(t *testing.T) {
	testutil.RunSuite(t, "Iterator Suite")
}
