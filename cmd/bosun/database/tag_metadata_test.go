package database

import (
	"testing"
	"time"

	"bosun.org/opentsdb"
)

func TestTagMetadata_RoundTrip(t *testing.T) {
	host := randString(4)
	tagset := opentsdb.TagSet{"host": host, "iface": "foo", "iname": "bar", "direction": "in"}
	if err := testData.PutTagMetadata(tagset, "alias", "foo", time.Now()); err != nil {
		t.Fatal(err)
	}
	metas, err := testData.GetTagMetadata(tagset, "alias")
	if err != nil {
		t.Fatal(err)
	}
	if len(metas) != 1 {
		t.Fatal("expected 1 metadata result")
	}
	m := metas[0]
	if m.Name != "alias" {
		t.Fatalf("name %s != alias", m.Name)
	}
	if !m.Tags.Equal(tagset) {
		t.Fatalf("tagset %s != %s", m.Tags.String(), tagset.String())
	}
}
