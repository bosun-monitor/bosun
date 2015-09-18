package database

import (
	"testing"
	"time"

	"bosun.org/opentsdb"
)

func TestTagMetadata_RoundTrip(t *testing.T) {
	tagset := opentsdb.TagSet{"host": "ny-web01", "iface": "foo", "iname": "bar", "direction": "in"}
	if err := testData.PutTagMetadata(tagset, "alias", "foo", time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := testData.PutTagMetadata(tagset, "mac", "aaaa", time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := testData.PutTagMetadata(opentsdb.TagSet{"host": "ny-web01", "iface": "foo"}, "aaaa", "bbbb", time.Now()); err != nil {
		t.Fatal(err)
	}
}
