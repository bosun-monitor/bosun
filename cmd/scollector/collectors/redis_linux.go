package collectors

import (
	"fmt"
	"io/ioutil"
	"regexp"
	"strings"
	"time"

	"github.com/garyburd/redigo/redis"

	"github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/slog"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_redis_linux})
}

var FIELDS_REDIS = map[string]bool{
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
	"total_commands_processed":     true,
	"total_connections_received":   true,
	"uptime_in_seconds":            true,
	"used_cpu_sys":                 true,
	"used_cpu_user":                true,
	"used_memory":                  true,
	"used_memory_rss":              true,
}

//For master_link_status
var MLS_MAP = map[string]string{
	"up":   "1",
	"down": "0",
}

//For aof_last_bgrewrite_status, rdb_last_bgsave_status
func status(s string) string {
	if s == "ok" {
		return "1"
	}
	return "0"
}

var (
	tcRE           = regexp.MustCompile(`^\s*#\s*scollector.(\w+)\s*=\s*(.+)$`)
	redisInstances map[string]string
)

func init() {
	update := func() {
		ri := make(map[string]string)
		oldRedis := false
		add := func(port, pid string) {
			cluster := fmt.Sprintf("port-%s", port)
			f, err := ioutil.ReadFile(fmt.Sprintf("/proc/%s/cmdline", pid))
			if err != nil {
				return
			}
			fsp := strings.Split(strings.Split(string(f), "\n")[0], "\u0000")
			if len(fsp) < 2 {
				return
			}
			cfg := fsp[len(fsp)-2]
			readLine(cfg, func(cfgline string) {
				result := tcRE.FindStringSubmatch(cfgline)
				if len(result) > 2 && strings.ToLower(result[0]) == "cluster" {
					cluster = strings.ToLower(result[1])
				}
			})
			ri[port] = cluster
		}
		readCommand(func(line string) {
			sp := strings.Fields(line)
			if len(sp) != 3 || !strings.HasSuffix(sp[1], "redis-server") {
				return
			}
			if !strings.Contains(sp[2], ":") {
				oldRedis = true
				return
			}
			pid := sp[0]
			port := strings.Split(sp[2], ":")[1]
			add(port, pid)
		}, "ps", "-e", "-o", "pid,args")
		if oldRedis {
			readCommand(func(line string) {
				if !strings.Contains(line, "redis-server") {
					return
				}
				sp := strings.Fields(line)
				if len(sp) < 7 || !strings.Contains(sp[3], ":") {
					return
				}
				pid := strings.Split(sp[6], "/")[0]
				port := strings.Split(sp[3], ":")[1]
				add(port, pid)
			}, "netstat", "-tnlp")
		}
		redisInstances = ri
	}
	update()
	go func() {
		for _ = range time.Tick(time.Minute * 5) {
			update()
		}
	}()
}

func c_redis_linux() opentsdb.MultiDataPoint {
	var md opentsdb.MultiDataPoint
	for port, cluster := range redisInstances {
		c, err := redis.Dial("tcp", fmt.Sprintf(":%s", port))
		if err != nil {
			slog.Infoln(err)
			continue
		}
		defer c.Close()
		tags := opentsdb.TagSet{
			"cluster": cluster,
			"port":    port,
		}
		lines, err := c.Do("INFO")
		if err != nil {
			slog.Infoln(err)
			continue
		}
		_ = tags
		for _, line := range strings.Split(string(lines.([]uint8)), "\n") {
			line = strings.TrimSpace(line)
			sp := strings.Split(line, ":")
			if len(sp) < 2 || !FIELDS_REDIS[sp[0]] {
				continue
			}
			if sp[0] == "master_link_status" {
				Add(&md, "redis."+sp[0], MLS_MAP[sp[1]], tags)
				continue
			}
			if sp[0] == "aof_last_bgrewrite_status" || sp[0] == "rdb_last_bgsave_status" {
				Add(&md, "redis."+sp[0], status(sp[1]), tags)
				continue
			}
			Add(&md, "redis."+sp[0], sp[1], tags)
		}
	}
	return md
}
