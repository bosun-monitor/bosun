package collectors

import (
	"strconv"
	"strings"

	"github.com/StackExchange/tcollector/opentsdb"
)

func init() {
	collectors = append(collectors, Collector{F: c_iostat_linux})
}

var FIELDS_DISK = []string{
	"read_requests",       // Total number of reads completed successfully.
	"read_merged",         // Adjacent read requests merged in a single req.
	"read_sectors",        // Total number of sectors read successfully.
	"msec_read",           // Total number of ms spent by all reads.
	"write_requests",      // Total number of writes completed successfully.
	"write_merged",        // Adjacent write requests merged in a single req.
	"write_sectors",       // Total number of sectors written successfully.
	"msec_write",          // Total number of ms spent by all writes.
	"ios_in_progress",     // Number of actual I/O requests currently in flight.
	"msec_total",          // Amount of time during which ios_in_progress >= 1.
	"msec_weighted_total", // Measure of recent I/O completion time and backlog.
}

var FIELDS_PART = []string{
	"read_issued",
	"read_sectors",
	"write_issued",
	"write_sectors",
}

func c_iostat_linux() opentsdb.MultiDataPoint {
	var md opentsdb.MultiDataPoint
	readProc("/proc/diskstats", func(s string) {
		values := strings.Fields(s)
		if len(values) < 4 {
			return
		} else if values[3] == "0" {
			// Skip disks that haven't done a single read.
			return
		}
		metric := "iostat.part."
		i0, _ := strconv.Atoi(values[0])
		i1, _ := strconv.Atoi(values[1])
		if i1%16 == 0 && i0 > 1 {
			metric = "iostat.disk."
		}
		device := values[2]
		if len(values) == 14 {
			for i, v := range values[3:] {
				Add(&md, metric+FIELDS_DISK[i], v, opentsdb.TagSet{"dev": device})
			}
		} else if len(values) == 7 {
			for i, v := range values[3:] {
				Add(&md, metric+FIELDS_PART[i], v, opentsdb.TagSet{"dev": device})
			}
		} else {
			l.Println("iostat: cannot parse")
		}
	})
	return md
}
