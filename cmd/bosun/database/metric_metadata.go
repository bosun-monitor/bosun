package database

import (
	"fmt"
	"time"

	"github.com/garyburd/redigo/redis"
)

func metricMetaKey(metric string) string {
	return fmt.Sprintf("mmeta:%s", metric)
}

func (d *dataAccess) hexpire() string {
	if d.isRedis {
		return "EXPIRE"
	}
	return "HEXPIRE"
}

const metricMetaTTL = int((time.Hour * 24 * 7) / time.Second)

func (d *dataAccess) PutMetricMetadata(metric string, field string, value string) error {
	conn := d.getConnection()
	defer conn.Close()
	_, err := conn.Do("HSET", metricMetaKey(metric), field, value)
	if err != nil {
		return err
	}
	_, err = conn.Do(d.hexpire(), metricMetaKey(metric), int32(metricMetaTTL))
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
