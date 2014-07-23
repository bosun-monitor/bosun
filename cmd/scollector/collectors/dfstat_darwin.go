package collectors

import (
	"strconv"
	"strings"

	"github.com/StackExchange/scollector/metadata"
	"github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/scollector/util"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_dfstat_darwin})
}

func c_dfstat_darwin() (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	util.ReadCommand(func(line string) {
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
		Add(&md, "darwin.disk.fs.total", fields[1], tags, metadata.Unknown, metadata.None, "")
		Add(&md, "darwin.disk.fs.used", fields[2], tags, metadata.Unknown, metadata.None, "")
		Add(&md, "darwin.disk.fs.free", fields[3], tags, metadata.Unknown, metadata.None, "")
		Add(&md, "darwin.disk.fs.inodes.total", f5+f6, tags, metadata.Unknown, metadata.None, "")
		Add(&md, "darwin.disk.fs.inodes.used", fields[5], tags, metadata.Unknown, metadata.None, "")
		Add(&md, "darwin.disk.fs.inodes.free", fields[6], tags, metadata.Unknown, metadata.None, "")
	}, "df", "-lki")
	return md, nil
}
