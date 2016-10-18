package collectors

import (
	"regexp"
	"strings"
	"time"

	"bosun.org/opentsdb"
	"bosun.org/util"
)

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
