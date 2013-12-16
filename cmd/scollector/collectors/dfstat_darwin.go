package collectors

import (
	"strconv"
	"strings"

	"github.com/StackExchange/tcollector/opentsdb"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_dfstat_darwin})
}

func c_dfstat_darwin() opentsdb.MultiDataPoint {
	var md opentsdb.MultiDataPoint
	readCommand(func(line string) {
		fields := strings.Fields(line)
		if line == "" || len(fields) < 9 || !IsDigit(fields[2]) {
			return
		}
		mount := fields[8]
		if strings.HasPrefix(mount, "/Volumes/Time Machine Backups") {
			return
		}
		f5, _ := strconv.Atoi(fields[5])
		f6, _ := strconv.Atoi(fields[6])
		tags := opentsdb.TagSet{"mount": mount}
		Add(&md, "df.1kblocks.total", fields[1], tags)
		Add(&md, "df.1kblocks.used", fields[2], tags)
		Add(&md, "df.1kblocks.free", fields[3], tags)
		Add(&md, "df.inodes.total", f5+f6, tags)
		Add(&md, "df.inodes.used", fields[5], tags)
		Add(&md, "df.inodes.free", fields[6], tags)
	}, "df", "-lki")
	return md
}
