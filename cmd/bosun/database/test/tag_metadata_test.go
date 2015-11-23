package dbtest

import (
	"testing"
	"time"

	"bosun.org/opentsdb"
)

func TestTagMetadata_RoundTrip(t *testing.T) {
	host := randString(4)
	tagset := opentsdb.TagSet{"host": host, "iface": "foo", "iname": "bar", "direction": "in"}
	if err := testData.Metadata().PutTagMetadata(tagset, "alias", "foo", time.Now()); err != nil {
		t.Fatal(err)
	}
	metas, err := testData.Metadata().GetTagMetadata(tagset, "alias")
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

func TestTagMetadata_SingleOrEmptyKey(t *testing.T) {
	host := randString(4)
	tagset := opentsdb.TagSet{"host": host, "iface": "foo"}
	if err := testData.Metadata().PutTagMetadata(tagset, "a", "a", time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := testData.Metadata().PutTagMetadata(tagset, "b", "b", time.Now()); err != nil {
		t.Fatal(err)
	}
	metas, err := testData.Metadata().GetTagMetadata(tagset, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(metas) != 2 {
		t.Fatalf("Expected 2 metadata entries for empty key. Got %d", len(metas))
	}
	metas, err = testData.Metadata().GetTagMetadata(tagset, "a")
	if err != nil {
		t.Fatal(err)
	}
	if len(metas) != 1 {
		t.Fatalf("Expected 1 metadata entry for provided key. Got %d", len(metas))
	}
}

func TestTagMetadata_Delete(t *testing.T) {
	host := randString(4)
	tagset := opentsdb.TagSet{"host": host, "iface": "foo"}
	if err := testData.Metadata().PutTagMetadata(tagset, "a", "a", time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := testData.Metadata().PutTagMetadata(tagset, "b", "b", time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := testData.Metadata().DeleteTagMetadata(tagset, "b"); err != nil {
		t.Fatal(err)
	}
	metas, err := testData.Metadata().GetTagMetadata(tagset, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(metas) != 1 {
		t.Fatalf("Expected 1 metadata entry for empty key. Got %d", len(metas))
	}
}
