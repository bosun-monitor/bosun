// +build darwin linux

package collectors

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"bosun.org/Godeps/_workspace/src/github.com/garyburd/redigo/redis"

	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"bosun.org/util"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_redis, init: redisInit})
}

var redisFields = map[string]bool{
	"aof_enabled":                  true,
	"aof_rewrite_in_progress":      true,
	"aof_rewrite_scheduled":        true,
	"aof_last_rewrite_time_sec":    true,
	"aof_current_rewrite_time_sec": true,
	"aof_last_bgrewrite_status":    true,
	"bgrewriteaof_in_progress":     true,
	"bgsave_in_progress":           true,
	"blocked_clients":              true,
	"changes_since_last_save":      true,
	"client_biggest_input_buf":     true,
	"client_longest_output_list":   true,
	"connected_clients":            true,
	"connected_slaves":             true,
	"evicted_keys":                 true,
	"expired_keys":                 true,
	"hash_max_zipmap_entries":      true,
	"hash_max_zipmap_value":        true,
	"keyspace_hits":                true,
	"keyspace_misses":              true,
	"master_link_status":           true,
	"master_sync_in_progress":      true,
	"master_last_io_seconds_ago":   true,
	"master_sync_left_bytes":       true,
	"mem_fragmentation_ratio":      true,
	"pubsub_channels":              true,
	"pubsub_patterns":              true,
	"rdb_changes_since_last_save":  true,
	"rdb_bgsave_in_progress":       true,
	"rdb_last_save_time":           true,
	"rdb_last_bgsave_status":       true,
	"rdb_last_bgsave_time_sec":     true,
	"rdb_current_bgsave_time_sec":  true,
	"role": true,
	"total_commands_processed":   true,
	"total_connections_received": true,
	"uptime_in_seconds":          true,
	"used_cpu_sys":               true,
	"used_cpu_user":              true,
	"used_memory":                true,
	"used_memory_rss":            true,
}

// For master_link_status.
var redisMlsMap = map[string]string{
	"up":   "1",
	"down": "0",
}

// For aof_last_bgrewrite_status, rdb_last_bgsave_status.
func status(s string) string {
	if s == "ok" {
		return "1"
	}
	return "0"
}

// For role which translates to is_slave
func slave(s string) string {
	if s == "slave" {
		return "1"
	}
	return "0"
}

var (
	tcRE           = regexp.MustCompile(`^\s*#\s*scollector.(\w+)\s*=\s*(.+)$`)
	redisInstances []opentsdb.TagSet
)

func redisScollectorTags(cfg string) map[string]string {
	m := make(opentsdb.TagSet)
	readLine(cfg, func(cfgline string) error {
		result := tcRE.FindStringSubmatch(cfgline)
		if len(result) == 3 {
			m[result[1]] = result[2]
		}
		return nil
	})
	return m
}

func redisInit() {
	update := func() {
		var instances []opentsdb.TagSet
		oldRedis := false
		add := func(port string) {
			ri := make(opentsdb.TagSet)
			ri["port"] = port
			instances = append(instances, ri)
		}
		util.ReadCommand(func(line string) error {
			sp := strings.Fields(line)
			if len(sp) != 3 || !strings.HasSuffix(sp[1], "redis-server") {
				return nil
			}
			if !strings.Contains(sp[2], ":") {
				oldRedis = true
				return nil
			}
			port := strings.Split(sp[2], ":")[1]
			if port != "0" {
				add(port)
			}
			return nil
		}, "ps", "-e", "-o", "pid,args")
		if oldRedis {
			util.ReadCommand(func(line string) error {
				if !strings.Contains(line, "redis-server") {
					return nil
				}
				sp := strings.Fields(line)
				if len(sp) < 7 || !strings.Contains(sp[3], ":") {
					return nil
				}
				port := strings.Split(sp[3], ":")[1]
				add(port)
				return nil
			}, "netstat", "-tnlp")
		}
		redisInstances = instances
	}
	update()
	go func() {
		for range time.Tick(time.Minute * 5) {
			update()
		}
	}()
}

func redisKeyCount(line string) (int64, error) {
	err := fmt.Errorf("Error parsing keyspace line from redis info: %v", line)
	colSplit := strings.Split(line, ":")
	if len(colSplit) < 2 {
		return 0, err
	}
	comSplit := strings.Split(colSplit[1], ",")
	if len(comSplit) != 3 {
		return 0, err
	}
	eqSplit := strings.Split(comSplit[0], "=")
	if len(eqSplit) != 2 || eqSplit[0] != "keys" {
		return 0, err
	}
	v, err := strconv.ParseInt(eqSplit[1], 10, 64)
	if err != nil {
		return 0, err
	}
	return v, nil
}

func c_redis() (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	var Error error
	for _, instance := range redisInstances {
		c, err := redis.Dial("tcp", fmt.Sprintf(":%s", instance["port"]))
		if err != nil {
			Error = err
			continue
		}
		defer c.Close()
		info, err := c.Do("info", "all")
		if err != nil {
			Error = err
			continue
		}
		tags := instance.Copy()
		infoSplit := strings.Split(string(info.([]uint8)), "\n")
		for _, line := range infoSplit {
			line = strings.TrimSpace(line)
			sp := strings.Split(line, ":")
			if len(sp) < 2 || sp[0] != "config_file" {
				continue
			}
			if sp[1] != "" {
				m := redisScollectorTags(sp[1])
				tags.Merge(m)
				break
			}
		}
		var keyspace bool
		var keys int64
		for _, line := range infoSplit {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if line == "# Keyspace" {
				keyspace = true
				continue
			}
			if keyspace {
				k, err := redisKeyCount(line)
				if err != nil {
					return nil, err
				}
				keys += k
				continue
			}
			sp := strings.Split(line, ":")
			if len(sp) < 2 || !(redisFields[sp[0]] || strings.HasPrefix(sp[0], "cmdstat_")) {
				continue
			}
			if sp[0] == "master_link_status" {
				Add(&md, "redis."+sp[0], redisMlsMap[sp[1]], tags, metadata.Unknown, metadata.None, "")
				continue
			}
			if sp[0] == "role" {
				Add(&md, "redis.is_slave", slave(sp[1]), tags, metadata.Gauge, metadata.Bool, "")
				continue
			}
			if sp[0] == "aof_last_bgrewrite_status" || sp[0] == "rdb_last_bgsave_status" {
				Add(&md, "redis."+sp[0], status(sp[1]), tags, metadata.Unknown, metadata.None, "")
				continue
			}
			if strings.HasPrefix(sp[0], "cmdstat_") {
				cmdStats := strings.Split(sp[1], ",")
				if len(cmdStats) < 3 {
					continue
				}
				cmdStatsCalls := strings.Split(cmdStats[0], "=")
				if len(cmdStatsCalls) < 2 {
					continue
				}
				cmdStatsUsec := strings.Split(cmdStats[1], "=")
				if len(cmdStatsUsec) < 2 {
					continue
				}
				var cmdStatsMsec, cmdStatsMsecPc float64
				microsec, err := strconv.ParseFloat(cmdStatsUsec[1], 64)
				if err != nil {
					continue
				}
				cmdStatsMsec = microsec / 1000
				cmdStatsUsecPc := strings.Split(cmdStats[2], "=")
				if len(cmdStatsUsecPc) < 2 {
					continue
				}
				microsec, err = strconv.ParseFloat(cmdStatsUsecPc[1], 64)
				if err != nil {
					continue
				}
				cmdStatsMsecPc = microsec / 1000
				if shortTag := strings.Split(sp[0], "_"); len(shortTag) == 2 {
					tags["cmd"] = shortTag[1]
				}
				Add(&md, "redis.cmdstats_msec_pc", cmdStatsMsecPc, tags, metadata.Gauge, metadata.MilliSecond, descRedisCmdMsecPc)
				Add(&md, "redis.cmdstats_msec", cmdStatsMsec, tags, metadata.Counter, metadata.MilliSecond, descRedisCmdMsec)
				Add(&md, "redis.cmdstats_calls", cmdStatsCalls[1], tags, metadata.Counter, metadata.Operation, descRedisCmdCalls)
				continue
			}
			Add(&md, "redis."+sp[0], sp[1], tags, metadata.Unknown, metadata.None, "")
		}
		Add(&md, "redis.key_count", keys, tags, metadata.Gauge, metadata.Key, descRedisKeyCount)
	}
	return md, Error
}

const (
	descRedisKeyCount  = "The total number of keys in the instance."
	descRedisCmdMsecPc = "Average CPU consumed per command execution."
	descRedisCmdMsec   = "Total CPU time consumed by commands."
	descRedisCmdCalls  = "Number of calls."
)
