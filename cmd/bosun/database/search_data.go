package database

import (
	"fmt"
	"strconv"

	"bosun.org/collect"
	"bosun.org/opentsdb"
	"bosun.org/slog"
	"bosun.org/util"
	"github.com/garyburd/redigo/redis"
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

func (d *dataAccess) Search() SearchDataAccess {
	return d
}

func (d *dataAccess) AddMetricForTag(tagK, tagV, metric string, time int64) error {
	defer collect.StartTimer("redis", opentsdb.TagSet{"op": "AddMetricForTag"})()
	conn := d.GetConnection()
	defer conn.Close()

	_, err := conn.Do("HSET", searchMetricKey(tagK, tagV), metric, time)
	return slog.Wrap(err)
}

func (d *dataAccess) GetMetricsForTag(tagK, tagV string) (map[string]int64, error) {
	defer collect.StartTimer("redis", opentsdb.TagSet{"op": "GetMetricsForTag"})()
	conn := d.GetConnection()
	defer conn.Close()

	return stringInt64Map(conn.Do("HGETALL", searchMetricKey(tagK, tagV)))
}

func stringInt64Map(d interface{}, err error) (map[string]int64, error) {
	if err != nil {
		return nil, err
	}
	vals, err := redis.Strings(d, err)
	if err != nil {
		return nil, err
	}
	result := make(map[string]int64)
	for i := 1; i < len(vals); i += 2 {
		time, _ := strconv.ParseInt(vals[i], 10, 64)
		result[vals[i-1]] = time
	}
	return result, slog.Wrap(err)
}

func (d *dataAccess) AddTagKeyForMetric(metric, tagK string, time int64) error {
	defer collect.StartTimer("redis", opentsdb.TagSet{"op": "AddTagKeyForMetric"})()
	conn := d.GetConnection()
	defer conn.Close()

	_, err := conn.Do("HSET", searchTagkKey(metric), tagK, time)
	return slog.Wrap(err)
}

func (d *dataAccess) GetTagKeysForMetric(metric string) (map[string]int64, error) {
	defer collect.StartTimer("redis", opentsdb.TagSet{"op": "GetTagKeysForMetric"})()
	conn := d.GetConnection()
	defer conn.Close()

	return stringInt64Map(conn.Do("HGETALL", searchTagkKey(metric)))
}

func (d *dataAccess) AddMetric(metric string, time int64) error {
	defer collect.StartTimer("redis", opentsdb.TagSet{"op": "AddMetric"})()
	conn := d.GetConnection()
	defer conn.Close()

	_, err := conn.Do("HSET", searchAllMetricsKey, metric, time)
	return slog.Wrap(err)
}
func (d *dataAccess) GetAllMetrics() (map[string]int64, error) {
	defer collect.StartTimer("redis", opentsdb.TagSet{"op": "GetAllMetrics"})()
	conn := d.GetConnection()
	defer conn.Close()

	return stringInt64Map(conn.Do("HGETALL", searchAllMetricsKey))
}

func (d *dataAccess) AddTagValue(metric, tagK, tagV string, time int64) error {
	defer collect.StartTimer("redis", opentsdb.TagSet{"op": "AddTagValue"})()
	conn := d.GetConnection()
	defer conn.Close()

	_, err := conn.Do("HSET", searchTagvKey(metric, tagK), tagV, time)
	return slog.Wrap(err)
}

func (d *dataAccess) GetTagValues(metric, tagK string) (map[string]int64, error) {
	defer collect.StartTimer("redis", opentsdb.TagSet{"op": "GetTagValues"})()
	conn := d.GetConnection()
	defer conn.Close()

	return stringInt64Map(conn.Do("HGETALL", searchTagvKey(metric, tagK)))
}

func (d *dataAccess) AddMetricTagSet(metric, tagSet string, time int64) error {
	defer collect.StartTimer("redis", opentsdb.TagSet{"op": "AddMetricTagSet"})()
	conn := d.GetConnection()
	defer conn.Close()

	_, err := conn.Do("HSET", searchMetricTagSetKey(metric), tagSet, time)
	return slog.Wrap(err)
}

func (d *dataAccess) GetMetricTagSets(metric string, tags opentsdb.TagSet) (map[string]int64, error) {
	defer collect.StartTimer("redis", opentsdb.TagSet{"op": "GetMetricTagSets"})()
	conn := d.GetConnection()
	defer conn.Close()

	var cursor = "0"
	result := map[string]int64{}

	for {
		vals, err := redis.Values(conn.Do(d.HSCAN(), searchMetricTagSetKey(metric), cursor))
		if err != nil {
			return nil, slog.Wrap(err)
		}
		cursor, err = redis.String(vals[0], nil)
		if err != nil {
			return nil, slog.Wrap(err)
		}
		mtss, err := stringInt64Map(vals[1], nil)
		if err != nil {
			return nil, slog.Wrap(err)
		}
		for mts, t := range mtss {
			ts, err := opentsdb.ParseTags(mts)
			if err != nil {
				return nil, slog.Wrap(err)
			}
			if ts.Subset(tags) {
				result[mts] = t
			}
		}

		if cursor == "" || cursor == "0" {
			break
		}
	}
	return result, nil
}

func (d *dataAccess) BackupLastInfos(m map[string]map[string]*LastInfo) error {
	defer collect.StartTimer("redis", opentsdb.TagSet{"op": "BackupLast"})()
	conn := d.GetConnection()
	defer conn.Close()

	dat, err := util.MarshalGzipJson(m)
	if err != nil {
		return slog.Wrap(err)
	}
	_, err = conn.Do("SET", "search:last", dat)
	return slog.Wrap(err)
}

func (d *dataAccess) LoadLastInfos() (map[string]map[string]*LastInfo, error) {
	defer collect.StartTimer("redis", opentsdb.TagSet{"op": "LoadLast"})()
	conn := d.GetConnection()
	defer conn.Close()

	b, err := redis.Bytes(conn.Do("GET", "search:last"))
	if err != nil {
		return nil, slog.Wrap(err)
	}
	var m map[string]map[string]*LastInfo
	err = util.UnmarshalGzipJson(b, &m)
	if err != nil {
		return nil, slog.Wrap(err)
	}
	return m, nil
}
