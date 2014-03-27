package collectors

import (
	"strings"
	"time"

	"github.com/StackExchange/scollector/opentsdb"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: yum_update_stats_linux, Interval: time.Minute * 5})
}

func yum_update_stats_linux() opentsdb.MultiDataPoint {
	var md opentsdb.MultiDataPoint
	regular_c := 0
	kernel_c := 0
	readCommand(func(line string) {
		fields := strings.Fields(line)
		if len(fields) > 1 && !strings.HasPrefix(fields[0], "Updated Packages") {
			if strings.HasPrefix(fields[0], "kern") {
				kernel_c++
			} else {
				regular_c++
			}
		}

	}, "yum", "list", "updates", "-q")
	Add(&md, "linux.updates.count", regular_c, opentsdb.TagSet{"type": "non-kernel"})
	Add(&md, "linux.updates.count", kernel_c, opentsdb.TagSet{"type": "kernel"})
	return md
}
