package database

import (
	"fmt"
	"time"

	"github.com/garyburd/redigo/redis"
)

func metricMetaKey(metric string) string {
	return fmt.Sprintf("mmeta:%s", metric)
}

const metricMetaTTL = int((time.Hour * 24 * 7) / time.Second)

func (d *dataAccess) PutMetricMetadata(metric string, field string, value string) error {
	if field != "desc" && field != "unit" && field != "rate" {
		return fmt.Errorf("Unknown metric metadata field: %s", field)
	}
	conn := d.getConnection()
	defer conn.Close()
	_, err := conn.Do("HSET", metricMetaKey(metric), field, value)
	_, err = conn.Do("HSET", metricMetaKey(metric), "lastTouched", time.Now().UTC().Unix())
	return err
}

func (d *dataAccess) GetMetricMetadata(metric string) (*MetricMetadata, error) {
	conn := d.getConnection()
	defer conn.Close()
	v, err := redis.Values(conn.Do("HGETALL", metricMetaKey(metric)))
	if err != nil {
		return nil, err
	}
	mm := &MetricMetadata{}
	if err := redis.ScanStruct(v, mm); err != nil {
		return nil, err
	}
	return mm, nil
}
