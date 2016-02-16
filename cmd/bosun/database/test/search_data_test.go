package dbtest

import (
	"fmt"
	"testing"

	"bosun.org/opentsdb"
)

func TestSearch_Metric_RoundTrip(t *testing.T) {
	host := randString(5)
	err := testData.Search().AddMetricForTag("host", host, "os.cpu", 42)
	if err != nil {
		t.Fatal(err)
	}

	metrics, err := testData.Search().GetMetricsForTag("host", host)
	if err != nil {
		t.Fatal(err)
	}
	if time, ok := metrics["os.cpu"]; !ok {
		t.Fatal("Expected to find os.cpu. I didn't.")
	} else if time != 42 {
		t.Fatalf("Expected timestamp of 42. Got %d", time)
	}
}

func TestSearch_MetricTagSets(t *testing.T) {
	if err := testData.Search().AddMetricTagSet("services.status", "host=abc,service=def", 42); err != nil {
		t.Fatal(err)
	}
	if err := testData.Search().AddMetricTagSet("os.cpu", "host=abc,service=def", 42); err != nil {
		t.Fatal(err)
	}
	if err := testData.Search().AddMetricTagSet("services.status", "host=abc,service=ghi", 42); err != nil {
		t.Fatal(err)
	}
	if err := testData.Search().AddMetricTagSet("services.status", "host=rrr,service=def", 42); err != nil {
		t.Fatal(err)
	}
	tagsets, err := testData.Search().GetMetricTagSets("services.status", opentsdb.TagSet{"host": "abc"})
	if err != nil {
		t.Fatal(err)
	}
	if len(tagsets) != 2 {
		t.Fatalf("Expected 2 tagsets. Found %d.", len(tagsets))
	}
}

func TestSearch_MetricTagSets_Scan(t *testing.T) {
	for i := int64(0); i < 10000; i++ {
		if err := testData.Search().AddMetricTagSet("metric", fmt.Sprintf("host=abc%d", i), i); err != nil {
			t.Fatal(err)
		}
	}
	tagsets, err := testData.Search().GetMetricTagSets("metric", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(tagsets) != 10000 {
		t.Fatalf("Expected 10000 tagsets. Found %d.", len(tagsets))
	}
}
