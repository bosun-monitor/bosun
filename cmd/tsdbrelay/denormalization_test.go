package main

import (
	"testing"

	"bosun.org/opentsdb"
)

func TestSimpleRewrite(t *testing.T) {
	rule := &DenormalizationRule{
		Metric:   "a.b.c",
		TagNames: []string{"host"},
	}
	dp := &opentsdb.DataPoint{
		Metric:    "a.b.c",
		Timestamp: 42,
		Value:     3,
		Tags:      opentsdb.TagSet{"host": "foo-bar", "baz": "qwerty"},
	}
	newDp, err := rule.Translate(dp)
	if err != nil {
		t.Fatal(err)
	}
	expectedName := "__foo-bar.a.b.c"
	if newDp.Metric != expectedName {
		t.Errorf("metric name %s is not `%s`", newDp.Metric, expectedName)
	}
	if newDp.Timestamp != dp.Timestamp {
		t.Errorf("new metric timestamp does not match. %d != %d", newDp.Timestamp, dp.Timestamp)
	}
	if newDp.Value != dp.Value {
		t.Errorf("new metric value does not match. %d != %d", newDp.Value, dp.Value)
	}
	if !dp.Tags.Equal(newDp.Tags) {
		t.Errorf("new metric tags do not match. %v != %v", newDp.Tags, dp.Tags)
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
	newDp, err := rule.Translate(dp)
	if err != nil {
		t.Fatal(err)
	}
	expectedName := "__foo-bar.eth0.a.b.c"
	if newDp.Metric != expectedName {
		t.Errorf("metric name %s is not `%s`", newDp.Metric, expectedName)
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
	_, err := rule.Translate(dp)
	if err == nil {
		t.Fatal("Expected error but got none.")
	}
}
