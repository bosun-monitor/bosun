package database

import (
	"bosun.org/collect"
	"bosun.org/opentsdb"
	"fmt"
	"strconv"

	"bosun.org/Godeps/_workspace/src/github.com/garyburd/redigo/redis"
)

/*
Search data in redis:

Metrics by tags:
search:metrics:{tagk}={tagv} -> hash of metric name to timestamp

Tag keys by metric:
search:tagk:{metric} -> hash of tag key to timestamp

Tag Values By metric/tag key
search:tagv:{metric}:{tagk} -> hash of tag value to timestamp
metric "__all__" is a special key that will hold all values for the tag key, regardless of metric

All Metrics:
search:allMetrics -> hash of metric name to timestamp
*/

const Search_All = "__all__"
const searchAllMetricsKey = "search:allMetrics"

func searchMetricKey(tagK, tagV string) string {
	return fmt.Sprintf("search:metrics:%s=%s", tagK, tagV)
}
func searchTagkKey(metric string) string {
	return fmt.Sprintf("search:tagk:%s", metric)
}
func searchTagvKey(metric, tagK string) string {
	return fmt.Sprintf("search:tagv:%s:%s", metric, tagK)
}
func searchMetricTagSetKey(metric string) string {
	return fmt.Sprintf("search:mts:%s", metric)
}

func (d *dataAccess) Search_AddMetricForTag(tagK, tagV, metric string, time int64) error {
	defer collect.StartTimer("redis", opentsdb.TagSet{"op": "AddMetricForTag"})()
	conn := d.GetConnection()
	defer conn.Close()

	_, err := conn.Do("HSET", searchMetricKey(tagK, tagV), metric, time)
	return err
}

func (d *dataAccess) Search_GetMetricsForTag(tagK, tagV string) (map[string]int64, error) {
	defer collect.StartTimer("redis", opentsdb.TagSet{"op": "GetMetricsForTag"})()
	conn := d.GetConnection()
	defer conn.Close()

	return stringInt64Map(conn.Do("HGETALL", searchMetricKey(tagK, tagV)))
}

func stringInt64Map(d interface{}, err error) (map[string]int64, error) {
	vals, err := redis.Strings(d, err)
	if err != nil {
		return nil, err
	}
	result := make(map[string]int64)
	for i := 1; i < len(vals); i += 2 {
		time, _ := strconv.ParseInt(vals[i], 10, 64)
		result[vals[i-1]] = time
	}
	return result, err
}

func (d *dataAccess) Search_AddTagKeyForMetric(metric, tagK string, time int64) error {
	defer collect.StartTimer("redis", opentsdb.TagSet{"op": "AddTagKeyForMetric"})()
	conn := d.GetConnection()
	defer conn.Close()

	_, err := conn.Do("HSET", searchTagkKey(metric), tagK, time)
	return err
}

func (d *dataAccess) Search_GetTagKeysForMetric(metric string) (map[string]int64, error) {
	defer collect.StartTimer("redis", opentsdb.TagSet{"op": "GetTagKeysForMetric"})()
	conn := d.GetConnection()
	defer conn.Close()

	return stringInt64Map(conn.Do("HGETALL", searchTagkKey(metric)))
}

func (d *dataAccess) Search_AddMetric(metric string, time int64) error {
	defer collect.StartTimer("redis", opentsdb.TagSet{"op": "AddMetric"})()
	conn := d.GetConnection()
	defer conn.Close()

	_, err := conn.Do("HSET", searchAllMetricsKey, metric, time)
	return err
}
func (d *dataAccess) Search_GetAllMetrics() (map[string]int64, error) {
	defer collect.StartTimer("redis", opentsdb.TagSet{"op": "GetAllMetrics"})()
	conn := d.GetConnection()
	defer conn.Close()

	return stringInt64Map(conn.Do("HGETALL", searchAllMetricsKey))
}

func (d *dataAccess) Search_AddTagValue(metric, tagK, tagV string, time int64) error {
	defer collect.StartTimer("redis", opentsdb.TagSet{"op": "AddTagValue"})()
	conn := d.GetConnection()
	defer conn.Close()

	_, err := conn.Do("HSET", searchTagvKey(metric, tagK), tagV, time)
	return err
}
func (d *dataAccess) Search_GetTagValues(metric, tagK string) (map[string]int64, error) {
	defer collect.StartTimer("redis", opentsdb.TagSet{"op": "GetTagValues"})()
	conn := d.GetConnection()
	defer conn.Close()

	return stringInt64Map(conn.Do("HGETALL", searchTagvKey(metric, tagK)))
}

func (d *dataAccess) Search_AddMetricTagSet(metric, tagSet string, time int64) error {
	defer collect.StartTimer("redis", opentsdb.TagSet{"op": "AddMetricTagSet"})()
	conn := d.GetConnection()
	defer conn.Close()

	_, err := conn.Do("HSET", searchMetricTagSetKey(metric), tagSet, time)
	return err
}
func (d *dataAccess) Search_GetMetricTagSets(metric string, tags opentsdb.TagSet) (map[string]int64, error) {
	defer collect.StartTimer("redis", opentsdb.TagSet{"op": "GetMetricTagSets"})()
	conn := d.GetConnection()
	defer conn.Close()

	mtss, err := stringInt64Map(conn.Do("HGETALL", searchMetricTagSetKey(metric)))
	if err != nil {
		return nil, err
	}
	for mts := range mtss {
		ts, err := opentsdb.ParseTags(mts)
		if err != nil {
			return nil, err
		}
		if !ts.Subset(tags) {
			delete(mtss, mts)
		}
	}
	return mtss, nil
}
