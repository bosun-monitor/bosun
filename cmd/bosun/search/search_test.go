package search

import (
	"os"
	"reflect"
	"testing"
	"time"

	"bosun.org/cmd/bosun/database/test"
	"bosun.org/opentsdb"
)

var testSearch *Search

func TestMain(m *testing.M) {
	testData, closeF := dbtest.StartTestRedis()
	testSearch = NewSearch(testData)
	status := m.Run()
	closeF()
	os.Exit(status)
}

func checkEqual(t *testing.T, err error, desc string, expected, actual []string) {
	if err != nil {
		t.Fatal(err)
	}
	if len(expected) != len(actual) {
		t.Fatalf("%s lengths differ. Expected %d, but found %d", desc, len(expected), len(actual))
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("Expect %s: %s. Found %s", desc, expected, actual)
	}
}

func TestIndex(t *testing.T) {
	mdp := opentsdb.MultiDataPoint{
		&opentsdb.DataPoint{Metric: "os.cpu", Value: 12.0, Timestamp: 13, Tags: opentsdb.TagSet{"host": "abc", "proc": "7"}},
		&opentsdb.DataPoint{Metric: "os.mem", Value: 4000, Timestamp: 13, Tags: opentsdb.TagSet{"host": "abc1"}},
		&opentsdb.DataPoint{Metric: "os.mem", Value: 4050, Timestamp: 13, Tags: opentsdb.TagSet{"host": "def"}},
		&opentsdb.DataPoint{Metric: "os.cpu2", Value: 12.0, Timestamp: 13, Tags: opentsdb.TagSet{"host": "abc"}},
	}
	testSearch.Index(mdp)
	time.Sleep(4 * time.Second)
	um, err := testSearch.UniqueMetrics()
	checkEqual(t, err, "metrics", []string{"os.cpu", "os.cpu2", "os.mem"}, um)

	tagks, err := testSearch.TagKeysByMetric("os.cpu")
	checkEqual(t, err, "tagk", []string{"host", "proc"}, tagks)

	tagvs, err := testSearch.TagValuesByTagKey("host", 0)
	checkEqual(t, err, "tagvsByTagKeyOnly", []string{"abc", "abc1", "def"}, tagvs)

	tagvs, err = testSearch.TagValuesByMetricTagKey("os.mem", "host", 0)
	checkEqual(t, err, "tagvsByTagKeyAndMetric", []string{"abc1", "def"}, tagvs)

	metrics, err := testSearch.MetricsByTagPair("host", "abc")
	checkEqual(t, err, "metricsByPair", []string{"os.cpu", "os.cpu2"}, metrics)
}
