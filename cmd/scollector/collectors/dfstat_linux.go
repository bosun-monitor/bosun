package collectors

import (
	"strings"

	"github.com/StackExchange/tcollector/opentsdb"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_dfstat_blocks_linux})
	collectors = append(collectors, &IntervalCollector{F: c_dfstat_inodes_linux})
}

func c_dfstat_blocks_linux() opentsdb.MultiDataPoint {
	var md opentsdb.MultiDataPoint
	readCommand(func(line string) {
		fields := strings.Fields(line)
		if line == "" || len(fields) < 6 || !IsDigit(fields[2]) {
			return
		}
		mount := fields[5]
		tags := opentsdb.TagSet{"mount": mount}
		Add(&md, "df.1kblocks.total", fields[1], tags)
		Add(&md, "df.1kblocks.used", fields[2], tags)
		Add(&md, "df.1kblocks.free", fields[3], tags)
	}, "df", "-lP")
	return md
}

func c_dfstat_inodes_linux() opentsdb.MultiDataPoint {
	var md opentsdb.MultiDataPoint
	readCommand(func(line string) {
		fields := strings.Fields(line)
		if line == "" || len(fields) < 6 || !IsDigit(fields[2]) {
			return
		}
		mount := fields[5]
		tags := opentsdb.TagSet{"mount": mount}
		Add(&md, "df.inodes.total", fields[1], tags)
		Add(&md, "df.inodes.used", fields[2], tags)
		Add(&md, "df.inodes.free", fields[3], tags)
	}, "df", "-liP")
	return md
}
