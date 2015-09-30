package dbtest

import (
	"testing"
)

func TestSearch_Metric_RoundTrip(t *testing.T) {
	host := randString(5)
	err := testData.Search_AddMetricForTag("host", host, "os.cpu", 42)
	if err != nil {
		t.Fatal(err)
	}

	metrics, err := testData.Search_GetMetricsForTag("host", host)
	if err != nil {
		t.Fatal(err)
	}
	if time, ok := metrics["os.cpu"]; !ok {
		t.Fatal("Expected to find os.cpu. I didn't.")
	} else if time != 42 {
		t.Fatalf("Expected timestamp of 42. Got %d", time)
	}
}
