package denormalize

import (
	"testing"

	"bosun.org/opentsdb"
)

func TestSimpleRewrite(t *testing.T) {
	rule := &DenormalizationRule{
		Metric:   "a.b.c",
		TagNames: []string{"host"},
	}
	tags := opentsdb.TagSet{"host": "foo-bar", "baz": "qwerty"}
	dp := &opentsdb.DataPoint{
		Metric:    "a.b.c",
		Timestamp: 42,
		Value:     3,
		Tags:      tags.Copy(),
	}
	err := rule.Translate(dp)
	if err != nil {
		t.Fatal(err)
	}
	expectedName := "__foo-bar.a.b.c"
	if dp.Metric != expectedName {
		t.Errorf("metric name %s is not `%s`", dp.Metric, expectedName)
	}
	if dp.Timestamp != 42 {
		t.Errorf("new metric timestamp does not match. %d != 42", dp.Timestamp)
	}
	if dp.Value != 3 {
		t.Errorf("new metric value does not match. %d != 3", dp.Value)
	}
	if !dp.Tags.Equal(tags) {
		t.Errorf("new metric tags do not match. %v != %v", dp.Tags, tags)
	}
}

func TestMultipleTags(t *testing.T) {
	rule := &DenormalizationRule{
		Metric:   "a.b.c",
		TagNames: []string{"host", "interface"},
	}
	dp := &opentsdb.DataPoint{
		Metric: "a.b.c",
		Tags:   opentsdb.TagSet{"host": "foo-bar", "interface": "eth0"},
	}
	err := rule.Translate(dp)
	if err != nil {
		t.Fatal(err)
	}
	expectedName := "__foo-bar.eth0.a.b.c"
	if dp.Metric != expectedName {
		t.Errorf("metric name %s is not `%s`", dp.Metric, expectedName)
	}
}

func TestRewrite_TagNotPresent(t *testing.T) {
	rule := &DenormalizationRule{
		Metric:   "a.b.c",
		TagNames: []string{"host"},
	}
	// Denormalization rule specified host, but data point has no host.
	// Return error on translate and don't send anything downstream.
	dp := &opentsdb.DataPoint{
		Metric:    "a.b.c",
		Timestamp: 42,
		Value:     3,
		Tags:      opentsdb.TagSet{"baz": "qwerty"},
	}
	err := rule.Translate(dp)
	if err == nil {
		t.Fatal("Expected error but got none.")
	}
}
