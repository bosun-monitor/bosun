package collectors

import (
	"fmt"

	"bosun.org/cmd/scollector/conf"
	"bosun.org/opentsdb"
)

func init() {
	registerInit(startRedisRemoteCollector)
}

func startRedisRemoteCollector(c *conf.Conf) {
	for _, r := range c.Redis {
		collectors = append(collectors, &IntervalCollector{
			F: func() (opentsdb.MultiDataPoint, error) {
				return c_redis(r)
			},
			name: fmt.Sprintf("%s-%s", "redis-remote", r.Address),
		})
	}
}
