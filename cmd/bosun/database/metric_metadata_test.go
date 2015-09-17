package database

import (
	"testing"
)

func TestMetricMetadataRoundTrip(t *testing.T) {
	if err := testData.PutMetricMetadata("os.cpu", "desc", "cpu of a server"); err != nil {
		t.Fatal(err)
	}
	if err := testData.PutMetricMetadata("os.cpu", "unit", "pct"); err != nil {
		t.Fatal(err)
	}
	meta, err := testData.GetMetricMetadata("os.cpu")
	if err != nil {
		t.Fatal(err)
	}
	if meta == nil {
		t.Fatal("did not find metadata I put in.")
	}
	if meta.Desc != "cpu of a server" {
		t.Fatal("Wrong description.")
	}
	if meta.Unit != "pct" {
		t.Fatal("Wrong Unit.")
	}
}
