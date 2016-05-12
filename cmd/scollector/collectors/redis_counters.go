package collectors

import (
	"fmt"
	"strconv"
	"strings"

	"bosun.org/cmd/scollector/conf"
	"bosun.org/collect"
	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"bosun.org/slog"
	"github.com/garyburd/redigo/redis"
)

func init() {
	registerInit(func(c *conf.Conf) {
		for _, red := range c.RedisCounters {
			collectors = append(collectors,
				&IntervalCollector{
					name: "redisCounters_" + red.Server,
					F: func() (opentsdb.MultiDataPoint, error) {
						return c_redis_counters(red.Server, red.Database)
					},
				})
		}
	})
}

func c_redis_counters(server string, db int) (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	conn, err := redis.Dial("tcp", server, redis.DialDatabase(db))
	if err != nil {
		return md, slog.Wrap(err)
	}
	defer conn.Close()
	cursor := 0
	for {
		vals, err := redis.Values(conn.Do("HSCAN", collect.RedisCountersKey, cursor))
		if err != nil {
			return md, slog.Wrap(err)
		}
		if len(vals) != 2 {
			return md, fmt.Errorf("Unexpected number of values")
		}
		cursor, err = redis.Int(vals[0], nil)
		if err != nil {
			return md, slog.Wrap(err)
		}
		pairs, err := redis.StringMap(vals[1], nil)
		if err != nil {
			return md, slog.Wrap(err)
		}
		for mts, val := range pairs {
			parts := strings.Split(mts, ":")
			if len(parts) != 2 {
				slog.Errorf("Invalid metric tag set counter: %s", mts)
				continue
			}
			metric := parts[0]
			tags, err := opentsdb.ParseTags(parts[1])
			if err != nil {
				slog.Errorf("Invalid tags: %s", parts[1])
				continue
			}
			v, err := strconv.Atoi(val)
			if err != nil {
				slog.Errorf("Invalid counter value: %s", val)
				continue
			}
			Add(&md, metric, v, tags, metadata.Counter, metadata.Count, "")
		}
		if cursor == 0 {
			break
		}
	}
	return md, nil
}
