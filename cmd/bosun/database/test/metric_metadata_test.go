package dbtest

import (
	"testing"
)

func TestMetricMetadata_RoundTrip(t *testing.T) {
	metric := randString(5)
	if err := testData.PutMetricMetadata(metric, "desc", "cpu of a server"); err != nil {
		t.Fatal(err)
	}
	if err := testData.PutMetricMetadata(metric, "unit", "pct"); err != nil {
		t.Fatal(err)
	}
	meta, err := testData.GetMetricMetadata(metric)
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

func TestMetricMetadata_NoneExists(t *testing.T) {
	meta, err := testData.GetMetricMetadata("asfaklsfjlkasjf")
	if err != nil {
		t.Fatal(err)
	}
	if meta != nil {
		t.Fatal("Should return nil if not exist")
	}
}

func TestMetricMetadata_BadField(t *testing.T) {
	if err := testData.PutMetricMetadata(randString(7), "desc1", "foo"); err == nil {
		t.Fatal("Expected failure to set bad metric metadata field")
	}
}
