package collectors

import (
	"fmt"
	"strconv"
	"strings"

	"bosun.org/cmd/scollector/conf"
	"bosun.org/collect"
	"bosun.org/metadata"
	"bosun.org/models"
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
						return cRedisCounters(red.Server, red.Database)
					},
				})
		}
	})
}

func cRedisCounters(server string, db int) (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	conn, err := redis.Dial("tcp", server, redis.DialDatabase(db))
	if err != nil {
		return md, slog.Wrap(err)
	}
	defer conn.Close()

	//do a dance to detect proper hscan command for ledis or redis
	hscanCmd := "XHSCAN"
	info, err := redis.String(conn.Do("info", "server"))
	if err != nil {
		return md, slog.Wrap(err)
	}
	if strings.Contains(info, "redis_version") {
		hscanCmd = "HSCAN"
	}

	cursor := "0"
	for {
		vals, err := redis.Values(conn.Do(hscanCmd, collect.RedisCountersKey, cursor))
		if err != nil {
			return md, slog.Wrap(err)
		}
		if len(vals) != 2 {
			return md, fmt.Errorf("Unexpected number of values")
		}
		cursor, err = redis.String(vals[0], nil)
		if err != nil {
			return md, slog.Wrap(err)
		}
		pairs, err := redis.StringMap(vals[1], nil)
		if err != nil {
			return md, slog.Wrap(err)
		}
		for key, val := range pairs {
			ak := models.AlertKey(key)

			v, err := strconv.Atoi(val)
			if err != nil {
				slog.Errorf("Invalid counter value: %s", val)
				continue
			}
			Add(&md, ak.Name(), v, ak.Group(), metadata.Counter, metadata.Count, "")
		}
		if cursor == "" || cursor == "0" {
			break
		}
	}
	return md, nil
}
