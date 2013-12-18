package collectors

import (
	"fmt"
	"io/ioutil"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/garyburd/redigo/redis"

	"github.com/StackExchange/tcollector/opentsdb"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_redis_linux})
}

var FIELDS_REDIS = map[string]bool{
	"bgrewriteaof_in_progress":   true,
	"bgsave_in_progress":         true,
	"blocked_clients":            true,
	"changes_since_last_save":    true,
	"client_biggest_input_buf":   true,
	"client_longest_output_list": true,
	"connected_clients":          true,
	"connected_slaves":           true,
	"evicted_keys":               true,
	"expired_keys":               true,
	"hash_max_zipmap_entries":    true,
	"hash_max_zipmap_value":      true,
	"keyspace_hits":              true,
	"keyspace_misses":            true,
	"mem_fragmentation_ratio":    true,
	"pubsub_channels":            true,
	"pubsub_patterns":            true,
	"total_commands_processed":   true,
	"total_connections_received": true,
	"uptime_in_seconds":          true,
	"used_cpu_sys":               true,
	"used_cpu_user":              true,
	"used_memory":                true,
	"used_memory_rss":            true,
}

var tcRE = regexp.MustCompile(`^\s*#\s*tcollector.(\w+)\s*=\s*(.+)$`)

var redisInstances map[int]string

func init() {
	update := func() {
		ri := make(map[int]string)
		readCommand(func(line string) {
			if !strings.Contains(line, "redis-server") {
				return
			}
			sp := strings.Fields(line)
			if len(sp) < 7 || !strings.Contains(sp[3], ":") {
				return
			}
			pid, _ := strconv.Atoi(strings.Split(sp[6], "/")[0])
			port, _ := strconv.Atoi(strings.Split(sp[3], ":")[1])
			cluster := fmt.Sprintf("port-%d", port)
			f, err := ioutil.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
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
		}, "netstat", "-tnlp")
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
		c, err := redis.Dial("tcp", fmt.Sprintf(":%d", port))
		if err != nil {
			l.Println(err)
		}
		defer c.Close()
		tags := opentsdb.TagSet{
			"cluster": cluster,
			"port":    strconv.Itoa(port),
		}
		lines, err := c.Do("INFO")
		if err != nil {
			l.Println(err)
			continue
		}
		_ = tags
		for _, line := range strings.Split(string(lines.([]uint8)), "\n") {
			line = strings.TrimSpace(line)
			sp := strings.Split(line, ":")
			if len(sp) < 2 || !FIELDS_REDIS[sp[0]] {
				continue
			}
			Add(&md, "redis."+sp[0], sp[1], tags)
		}
	}
	return md
}
