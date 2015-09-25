package database

import (
	"fmt"
	"time"

	"bosun.org/_third_party/github.com/garyburd/redigo/redis"
	"bosun.org/collect"
	"bosun.org/opentsdb"
)

/*
Metric metadata is the fields associated with every metric: desc, rate and unit.
They are stored as a simple hash structure:

mmeta:{{metric}} -> {desc:"",unit:"",rate:"",lastTouched:123}

lastTouched time is unix timestamp of last time this metric metadata was set.

*/

func metricMetaKey(metric string) string {
	return fmt.Sprintf("mmeta:%s", metric)
}

const metricMetaTTL = int((time.Hour * 24 * 7) / time.Second)

func (d *dataAccess) PutMetricMetadata(metric string, field string, value string) error {
	defer collect.StartTimer("redis", opentsdb.TagSet{"op": "PutMetricMeta"})()
	if field != "desc" && field != "unit" && field != "rate" {
		return fmt.Errorf("Unknown metric metadata field: %s", field)
	}
	conn := d.getConnection()
	defer conn.Close()
	_, err := conn.Do("HMSET", metricMetaKey(metric), field, value, "lastTouched", time.Now().UTC().Unix())
	return err
}

func (d *dataAccess) GetMetricMetadata(metric string) (*MetricMetadata, error) {
	defer collect.StartTimer("redis", opentsdb.TagSet{"op": "GetMetricMeta"})()
	conn := d.getConnection()
	defer conn.Close()
	v, err := redis.Values(conn.Do("HGETALL", metricMetaKey(metric)))
	if err != nil {
		return nil, err
	}
	if len(v) == 0 {
		return nil, nil
	}
	mm := &MetricMetadata{}
	if err := redis.ScanStruct(v, mm); err != nil {
		return nil, err
	}
	return mm, nil
}
