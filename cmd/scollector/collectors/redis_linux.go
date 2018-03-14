package collectors

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/garyburd/redigo/redis"

	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"bosun.org/util"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_redis, init: redisInit})
}

var redisMeta = map[string]MetricMeta{ // http://redis.io/commands/info)
	// Persistence Section
	//   AOF
	"aof_enabled": {
		RateType: metadata.Gauge,
		Unit:     metadata.Enabled,
		Desc:     "AOF Enabled indicates that Append Only File logging is activated.",
	},
	"aof_current_size": {
		RateType: metadata.Gauge,
		Unit:     metadata.Bytes,
		Desc:     "The current file size of the AOF (Append Only File).",
	},
	"aof_rewrite_in_progress": {
		RateType: metadata.Gauge,
		Unit:     metadata.InProgress,
		Desc:     "Rewrite in progress indicates that AOF (Append Only File) logging is activated.",
	},
	"aof_rewrite_scheduled": {
		RateType: metadata.Gauge,
		Unit:     metadata.Scheduled,
		Desc:     "AOF rewrite scheduled means an Append Only file rewrite operation will be scheduled once the on-going RDB save is complete.",
	},
	"aof_last_rewrite_time_sec": {
		RateType: metadata.Gauge,
		Unit:     metadata.Second,
		Desc:     "The duration of the last AOF (Append Only file) rewrite operation in seconds.",
	},
	"aof_current_rewrite_time_sec": {
		RateType: metadata.Gauge,
		Unit:     metadata.Second,
		Desc:     "The duration of the ongoing AOF (Append Only file) rewrite operation in seconds -- if there is one.",
	},
	"aof_last_bgrewrite_status": {
		RateType: metadata.Gauge,
		Unit:     metadata.Bool,
		Desc:     "The status of the last AOF (Append Only File) rewrite opperation.",
	},
	//   RDB
	"rdb_bgsave_in_progress": {
		RateType: metadata.Gauge,
		Unit:     metadata.InProgress,
		Desc:     "BGSAVE in progress indicates if a RDB save is on-going.",
	},
	"rdb_changes_since_last_save": {
		RateType: metadata.Gauge,
		Unit:     metadata.Change,
		Desc:     "The number of operations that produced some kind of changes in the dataset since the last time either SAVE or BGSAVE was called.",
	},
	"rdb_last_bgsave_status": {
		RateType: metadata.Gauge,
		Unit:     metadata.Bool,
		Desc:     "The Status of the last RDB save operation.",
	},
	"rdb_last_bgsave_time_sec": {
		RateType: metadata.Gauge,
		Unit:     metadata.Second,
		Desc:     "The duration of the last RDB save operation.",
	},
	"rdb_current_bgsave_time_sec": {
		RateType: metadata.Gauge,
		Unit:     metadata.Second,
		Desc:     "The duration of the ongoing RDB save operation -- if there is one.",
	},
	"rdb_last_save_time": {
		RateType: metadata.Gauge,
		Unit:     metadata.Timestamp,
		Desc:     "The epoch-based timestamp of last successful RDB save.",
	},

	// Clients Section
	"blocked_clients": {
		RateType: metadata.Gauge,
		Unit:     metadata.Client,
		Desc:     "The number of clients pending on a blocking call (BLPOP, BRPOP, BRPOPLPUSH).",
	},
	"connected_clients": {
		RateType: metadata.Gauge,
		Unit:     metadata.Connection,
		Desc:     "The number of client connections (excluding connections from slaves).",
	},
	"client_biggest_input_buf": {
		RateType: metadata.Gauge,
		Unit:     metadata.Count, // Need to figure out what this is, bytes?
		Desc:     "The biggest input buffer among current client connections.",
	},
	"client_longest_output_list": {
		RateType: metadata.Gauge,
		Unit:     metadata.Count, // Need to figure out what this is, length?
		Desc:     "The longest output list among current client connections.",
	},

	// Replication Sections
	"connected_slaves": {
		RateType: metadata.Gauge,
		Unit:     metadata.Slave,
		Desc:     "The number of connected slaves.",
	},
	"master_link_status": {
		RateType: metadata.Gauge,
		Unit:     metadata.Ok,
		Desc:     "The up/down status of the link to the master.",
	},
	"master_last_io_seconds_ago": {
		RateType: metadata.Gauge,
		Unit:     metadata.Second,
		Desc:     "The number of seconds since the last interaction with master.",
	},
	"master_sync_in_progress": {
		RateType: metadata.Gauge,
		Unit:     metadata.InProgress,
		Desc:     "Master sync in progress indicates that the master is syncing to the slave.",
	},
	"master_sync_left_bytes": {
		RateType: metadata.Gauge,
		Unit:     metadata.Bytes,
		Desc:     "The number of bytes left before syncing is complete.",
	},
	"master_sync_last_io_seconds_ago": {
		RateType: metadata.Gauge,
		Unit:     metadata.Second,
		Desc:     "The number of seconds since last transfer I/O during a SYNC operation.",
	},

	// Stats Section
	"evicted_keys": {
		RateType: metadata.Counter,
		Unit:     metadata.Key,
		Desc:     "The number of evicted keys due to maxmemory limit.",
	},
	"expired_keys": {
		RateType: metadata.Counter,
		Unit:     metadata.Key,
		Desc:     "The total total number of key expiration events.",
	},
	"keyspace_hits": {
		RateType: metadata.Counter,
		Unit:     metadata.CacheHit,
		Desc:     "The number of successful lookup of keys in the main dictionary.",
	},
	"keyspace_misses": {
		RateType: metadata.Counter,
		Unit:     metadata.CacheMiss,
		Desc:     "The number of failed lookup of keys in the main dictionary.",
	},
	"used_cpu_sys": {
		RateType: metadata.Counter,
		Unit:     metadata.Pct,
		Desc:     "The system CPU used by the main Redis process.",
	},
	"used_cpu_user": {
		RateType: metadata.Counter,
		Unit:     metadata.Pct,
		Desc:     "The user space CPU used by the main Redis process.",
	},
	"uptime_in_seconds": {
		RateType: metadata.Gauge,
		Unit:     metadata.Second,
		Desc:     "The number of seconds since Redis server start.",
	},
	"total_connections_received": {
		RateType: metadata.Counter,
		Unit:     metadata.Connection,
		Desc:     "The total number of connections accepted by the server.",
	},
	"total_commands_processed": {
		RateType: metadata.Counter,
		Unit:     metadata.Command,
		Desc:     "The total number of commands processed by the server.",
	},
	"pubsub_channels": {
		RateType: metadata.Gauge,
		Unit:     metadata.Channel,
		Desc:     "Global number of pub/sub channels with client subscriptions.",
	},
	"pubsub_patterns": {
		RateType: metadata.Gauge,
		Unit:     "Pattern",
		Desc:     "Global number of pub/sub channels with client subscriptions.",
	},
	"rejected_connections": {
		RateType: metadata.Counter,
		Unit:     metadata.Connection,
		Desc:     "The number of connections rejected because of maxclients limit.",
	},
	"sync_full": {
		RateType: metadata.Gauge, // Although the sync metrics are counters, it is not something by default you would want as a rate per second
		Unit:     metadata.Resync,
		Desc:     "The number of full resynchronizations with slaves.",
	},
	"sync_partial_ok": {
		RateType: metadata.Gauge,
		Unit:     metadata.Resync,
		Desc:     "The number of accepted PSYNC (partial resynchronization) requests.",
	},
	"sync_partial_err": {
		RateType: metadata.Gauge,
		Unit:     metadata.Resync,
		Desc:     "The number of unaccepted PSYNC (partial resynchronization) requests.",
	},

	// Memory Section
	"used_memory": {
		RateType: metadata.Gauge,
		Unit:     metadata.Bytes,
		Desc:     "The total number of bytes allocated by Redis using its allocator (either standard libc, jemalloc, or an alternative allocator such as tcmalloc.",
	},
	"used_memory_rss": {
		RateType: metadata.Gauge,
		Unit:     metadata.Bytes,
		Desc:     "The number of bytes that Redis allocated as seen by the operating system (a.k.a resident set size). This is the number reported by tools such as top(1) and ps(1).",
	},
	"mem_fragmentation_ratio": {
		RateType: metadata.Gauge,
		Unit:     metadata.Ratio,
		Desc:     "The ratio between used_memory_rss and used_memory.",
	},

	//Other
	"role": {}, // This gets treated independtly to create the is_slave metric
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
			if port != "0" && InContainer(sp[0]) == false {
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
			if len(sp) < 2 {
				continue
			}
			m, foundMeta := redisMeta[sp[0]]
			if !(foundMeta || strings.HasPrefix(sp[0], "cmdstat_")) {
				continue
			}
			if sp[0] == "master_link_status" {
				Add(&md, "redis."+sp[0], redisMlsMap[sp[1]], tags, m.RateType, m.Unit, m.Desc)
				continue
			}
			if sp[0] == "role" {
				Add(&md, "redis.is_slave", slave(sp[1]), tags, metadata.Gauge, metadata.Bool, descRedisIsSlave)
				continue
			}
			if sp[0] == "aof_last_bgrewrite_status" || sp[0] == "rdb_last_bgsave_status" {
				Add(&md, "redis."+sp[0], status(sp[1]), tags, m.RateType, m.Unit, m.Desc)
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
			Add(&md, "redis."+sp[0], sp[1], tags, m.RateType, m.Unit, m.Desc)
		}
		Add(&md, "redis.key_count", keys, tags, metadata.Gauge, metadata.Key, descRedisKeyCount)
	}
	return md, Error
}

const (
	descRedisKeyCount  = "The total number of keys in the instance."
	descRedisCmdMsecPc = "The average CPU consumed per command execution."
	descRedisCmdMsec   = "The total CPU time consumed by commands."
	descRedisCmdCalls  = "The total number of calls."
	descRedisIsSlave   = "This indicates if the redis instance is a slave or not."
)
