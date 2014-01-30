package collectors

import (
	"strconv"
	"strings"

	"github.com/StackExchange/scollector/opentsdb"
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
		os_tags := opentsdb.TagSet{"disk": mount}
		//Meta Data will need to indicate that these are 1kblocks
		Add(&md, "linux.disk.fs.space_total", fields[1], tags)
		Add(&md, "linux.disk.fs.space_used", fields[2], tags)
		Add(&md, "linux.disk.fs.space_free", fields[3], tags)
		Add(&md, "os.disk.fs.space_total", fields[1], tags)
		Add(&md, "os.disk.fs.space_used", fields[2], tags)
		Add(&md, "os.disk.fs.space_free", fields[3], tags)
		st, err := strconv.Atoi(fields[1])
		sf, err := strconv.Atoi(fields[3])
		if err == nil {
			if st != 0 {
				Add(&md, "os.disk.fs.percent_free", sf/st, os_tags)
			}
		}
	}, "df", "-lP", "--block-size", "1")
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
		Add(&md, "linux.disk.fs.inodes_total", fields[1], tags)
		Add(&md, "linux.disk.fs.inodes_used", fields[2], tags)
		Add(&md, "linux.disk.fs.inodes_free", fields[3], tags)
	}, "df", "-liP")
	return md
}
