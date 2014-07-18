package collectors

import (
	"strings"
	"time"

	"github.com/StackExchange/scollector/metadata"
	"github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/scollector/util"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: yum_update_stats_linux, Interval: time.Minute * 5})
}

func yum_update_stats_linux() opentsdb.MultiDataPoint {
	var md opentsdb.MultiDataPoint
	regular_c := 0
	kernel_c := 0
	err := util.ReadCommand(func(line string) {
		fields := strings.Fields(line)
		if len(fields) > 1 && !strings.HasPrefix(fields[0], "Updated Packages") {
			if strings.HasPrefix(fields[0], "kern") {
				kernel_c++
			} else {
				regular_c++
			}
		}

	}, "yum", "list", "updates", "-q")
	if err != nil {
		return nil
	}
	Add(&md, "linux.updates.count", regular_c, opentsdb.TagSet{"type": "non-kernel"}, metadata.Unknown, metadata.None, "")
	Add(&md, "linux.updates.count", kernel_c, opentsdb.TagSet{"type": "kernel"}, metadata.Unknown, metadata.None, "")
	return md
}
