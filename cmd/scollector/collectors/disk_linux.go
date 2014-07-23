package collectors

import (
	"io/ioutil"
	"regexp"
	"strconv"
	"strings"

	"github.com/StackExchange/scollector/metadata"
	"github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/scollector/util"
	"github.com/StackExchange/slog"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_iostat_linux})
	collectors = append(collectors, &IntervalCollector{F: c_dfstat_blocks_linux})
	collectors = append(collectors, &IntervalCollector{F: c_dfstat_inodes_linux})
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

func removable(major, minor string) bool {
	//We don't return an error, because removable may not exist for partitions of a removable device
	//So this is really "best effort" and we will have to see how it works in practice.
	b, err := ioutil.ReadFile("/sys/dev/block/" + major + ":" + minor + "/removable")
	if err != nil {
		return false
	}
	return strings.Trim(string(b), "\n") == "1"
	return false
}

var sdiskRE = regexp.MustCompile(`/dev/(sd[a-z])[0-9]?`)

func removable_fs(name string) bool {
	s := sdiskRE.FindStringSubmatch(name)
	if len(s) > 1 {
		b, err := ioutil.ReadFile("/sys/block/" + s[1] + "/removable")
		if err != nil {
			return false
		}
		return strings.Trim(string(b), "\n") == "1"
	}
	return false
}

func c_iostat_linux() (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	var removables []string
	readLine("/proc/diskstats", func(s string) {
		values := strings.Fields(s)
		if len(values) < 4 {
			return
		} else if values[3] == "0" {
			// Skip disks that haven't done a single read.
			return
		}
		metric := "linux.disk.part."
		i0, _ := strconv.Atoi(values[0])
		i1, _ := strconv.Atoi(values[1])
		if i1%16 == 0 && i0 > 1 {
			metric = "linux.disk."
		}
		device := values[2]
		ts := opentsdb.TagSet{"dev": device}
		if removable(values[0], values[1]) {
			removables = append(removables, device)
		}
		for _, r := range removables {
			if strings.HasPrefix(device, r) {
				metric += "rem."
			}
		}
		if len(values) == 14 {
			var read_sectors, msec_read, write_sectors, msec_write float64
			for i, v := range values[3:] {
				switch FIELDS_DISK[i] {
				case "read_sectors":
					read_sectors, _ = strconv.ParseFloat(v, 64)
				case "msec_read":
					msec_read, _ = strconv.ParseFloat(v, 64)
				case "write_sectors":
					write_sectors, _ = strconv.ParseFloat(v, 64)
				case "msec_write":
					msec_write, _ = strconv.ParseFloat(v, 64)
				}
				Add(&md, metric+FIELDS_DISK[i], v, ts, metadata.Unknown, metadata.None, "")
			}
			if read_sectors != 0 && msec_read != 0 {
				Add(&md, metric+"time_per_read", read_sectors/msec_read, ts, metadata.Unknown, metadata.None, "")
			}
			if write_sectors != 0 && msec_write != 0 {
				Add(&md, metric+"time_per_write", write_sectors/msec_write, ts, metadata.Unknown, metadata.None, "")
			}
		} else if len(values) == 7 {
			for i, v := range values[3:] {
				Add(&md, metric+FIELDS_PART[i], v, ts, metadata.Unknown, metadata.None, "")
			}
		} else {
			slog.Infoln("iostat: cannot parse")
		}
	})
	return md, nil
}

func c_dfstat_blocks_linux() (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	util.ReadCommand(func(line string) {
		fields := strings.Fields(line)
		if line == "" || len(fields) < 6 || !IsDigit(fields[2]) {
			return
		}
		fs := fields[0]
		mount := fields[5]
		tags := opentsdb.TagSet{"mount": mount}
		os_tags := opentsdb.TagSet{"disk": mount}
		metric := "linux.disk.fs."
		ometric := "os.disk.fs."
		if removable_fs(fs) {
			metric += "rem."
			ometric += "rem."
		}
		// Meta Data will need to indicate that these are 1kblocks.
		Add(&md, metric+"space_total", fields[1], tags, metadata.Unknown, metadata.None, "")
		Add(&md, metric+"space_used", fields[2], tags, metadata.Unknown, metadata.None, "")
		Add(&md, metric+"space_free", fields[3], tags, metadata.Unknown, metadata.None, "")
		Add(&md, ometric+"space_total", fields[1], os_tags, metadata.Unknown, metadata.None, "")
		Add(&md, ometric+"space_used", fields[2], os_tags, metadata.Unknown, metadata.None, "")
		Add(&md, ometric+"space_free", fields[3], os_tags, metadata.Unknown, metadata.None, "")
		st, _ := strconv.ParseFloat(fields[1], 64)
		sf, _ := strconv.ParseFloat(fields[3], 64)
		if st != 0 {
			Add(&md, osDiskPctFree, sf/st*100, os_tags, metadata.Unknown, metadata.None, "")
		}
	}, "df", "-lP", "--block-size", "1")
	return md, nil
}

func c_dfstat_inodes_linux() (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	util.ReadCommand(func(line string) {
		fields := strings.Fields(line)
		if len(fields) < 6 || !IsDigit(fields[2]) {
			return
		}
		mount := fields[5]
		fs := fields[0]
		tags := opentsdb.TagSet{"mount": mount}
		metric := "linux.disk.fs."
		if removable_fs(fs) {
			metric += "rem."
		}
		Add(&md, metric+"inodes_total", fields[1], tags, metadata.Unknown, metadata.None, "")
		Add(&md, metric+"inodes_used", fields[2], tags, metadata.Unknown, metadata.None, "")
		Add(&md, metric+"inodes_free", fields[3], tags, metadata.Unknown, metadata.None, "")
	}, "df", "-liP")
	return md, nil
}
