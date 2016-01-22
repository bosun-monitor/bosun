package collectors

import (
	"strconv"
	"strings"

	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"bosun.org/util"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_dfstat_darwin})
}

func c_dfstat_darwin() (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	util.ReadCommand(func(line string) error {
		fields := strings.Fields(line)
		if _, err := strconv.Atoi(fields[2]); line == "" || len(fields) < 9 || err != nil {
			return nil
		}
		mount := fields[8]
		if strings.HasPrefix(mount, "/Volumes/Time Machine Backups") {
			return nil
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
		return nil
	}, "df", "-lki")
	return md, nil
}
