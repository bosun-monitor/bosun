package sched

import (
	"fmt"
	"testing"
	"time"

	"bosun.org/metadata"
	"bosun.org/opentsdb"
)

var bm_sched_50k *Schedule
var bm_sched_2m *Schedule

func init() {
	bm_sched_50k = &Schedule{}
	val := &Metavalue{Time: time.Now(), Value: "foo"}

	bm_sched_50k.Metadata = map[metadata.Metakey]*Metavalue{
		{Tags: "host=host1", Name: "foo"}: val,
	}
	bm_sched_50k.metricMetadata = map[string]*MetadataMetric{}
	for i := 0; i < 50000; i++ {
		key := metadata.Metakey{Name: "foo"}
		key.Tags = fmt.Sprintf("host=host%d,somethingElse=aaa", i)
		bm_sched_50k.Metadata[key] = val

		bm_sched_50k.metricMetadata[fmt.Sprintf("m%d", i)] = &MetadataMetric{Description: "aaa"}
	}
	bm_sched_2m = &Schedule{}
	bm_sched_2m.Metadata = map[metadata.Metakey]*Metavalue{
		{Tags: "host=host1", Name: "foo"}: val,
	}
	for i := 0; i < 2000000; i++ {
		key := metadata.Metakey{Name: "foo"}
		key.Tags = fmt.Sprintf("host=host%d,somethingElse=aaa", i)
		bm_sched_2m.Metadata[key] = val
	}
}

func BenchmarkMetadataGet50K(b *testing.B) {
	for i := 0; i < b.N; i++ {
		bm_sched_50k.GetMetadata("", opentsdb.TagSet{"host": "host1"})
	}
}

func BenchmarkMetadataGet2M(b *testing.B) {
	for i := 0; i < b.N; i++ {
		bm_sched_2m.GetMetadata("", opentsdb.TagSet{"host": "host1"})
	}
}

func BenchmarkMetricMetadata(b *testing.B) {
	for i := 0; i < b.N; i++ {

	}
}
