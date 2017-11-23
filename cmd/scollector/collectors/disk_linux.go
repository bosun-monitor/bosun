package collectors

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"bosun.org/slog"
	"bosun.org/util"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_iostat_linux})
	collectors = append(collectors, &IntervalCollector{F: c_dfstat_blocks_linux, Interval: time.Second * 30})
	collectors = append(collectors, &IntervalCollector{F: c_dfstat_inodes_linux, Interval: time.Second * 30})
	collectors = append(collectors, &IntervalCollector{F: checkMdadmLinux, Interval: 1 * time.Minute})
}

var diskLinuxFields = []struct {
	key  string
	rate metadata.RateType
	unit metadata.Unit
	desc string
}{
	{"read_requests", metadata.Counter, metadata.Count, "Total number of reads completed successfully."},
	{"read_merged", metadata.Counter, metadata.Count, "Adjacent read requests merged in a single req."},
	{"read_sectors", metadata.Counter, metadata.Count, "Total number of sectors read successfully."},
	{"msec_read", metadata.Counter, metadata.MilliSecond, "Total number of ms spent by all reads."},
	{"write_requests", metadata.Counter, metadata.Count, "Total number of writes completed successfully."},
	{"write_merged", metadata.Counter, metadata.Count, " Adjacent write requests merged in a single req."},
	{"write_sectors", metadata.Counter, metadata.Count, "Total number of sectors written successfully."},
	{"msec_write", metadata.Counter, metadata.MilliSecond, "Total number of ms spent by all writes."},
	{"ios_in_progress", metadata.Gauge, metadata.Operation, "Number of actual I/O requests currently in flight."},
	{"msec_total", metadata.Counter, metadata.MilliSecond, "Amount of time during which ios_in_progress >= 1."},
	{"msec_weighted_total", metadata.Gauge, metadata.MilliSecond, "Measure of recent I/O completion time and backlog."},
}

var diskLinuxFieldsPart = []struct {
	key  string
	rate metadata.RateType
	unit metadata.Unit
}{
	{"read_issued", metadata.Counter, metadata.Count},
	{"read_sectors", metadata.Counter, metadata.Count},
	{"write_issued", metadata.Counter, metadata.Count},
	{"write_sectors", metadata.Counter, metadata.Count},
}

func removable(major, minor string) bool {
	//We don't return an error, because removable may not exist for partitions of a removable device
	//So this is really "best effort" and we will have to see how it works in practice.
	b, err := ioutil.ReadFile("/sys/dev/block/" + major + ":" + minor + "/removable")
	if err != nil {
		return false
	}
	return strings.Trim(string(b), "\n") == "1"
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

func isPseudoFS(name string) (res bool) {
	err := readLine("/proc/filesystems", func(s string) error {
		ss := strings.Split(s, "\t")
		if len(ss) == 2 && ss[1] == name && ss[0] == "nodev" {
			res = true
		}
		return nil
	})
	if err != nil {
		slog.Errorf("can not read '/proc/filesystems': %v", err)
	}
	return
}

func c_iostat_linux() (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	var removables []string
	err := readLine("/proc/diskstats", func(s string) error {
		values := strings.Fields(s)
		if len(values) < 4 {
			return nil
		} else if values[3] == "0" {
			// Skip disks that haven't done a single read.
			return nil
		}
		metric := "linux.disk.part."
		i0, _ := strconv.Atoi(values[0])
		i1, _ := strconv.Atoi(values[1])
		var block_size int64
		device := values[2]
		ts := opentsdb.TagSet{"dev": device}
		if i1%16 == 0 && i0 > 1 {
			metric = "linux.disk."
			if b, err := ioutil.ReadFile("/sys/block/" + device + "/queue/hw_sector_size"); err == nil {
				block_size, _ = strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64)
			}
		}
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
				switch diskLinuxFields[i].key {
				case "read_sectors":
					read_sectors, _ = strconv.ParseFloat(v, 64)
				case "msec_read":
					msec_read, _ = strconv.ParseFloat(v, 64)
				case "write_sectors":
					write_sectors, _ = strconv.ParseFloat(v, 64)
				case "msec_write":
					msec_write, _ = strconv.ParseFloat(v, 64)
				}
				Add(&md, metric+diskLinuxFields[i].key, v, ts, diskLinuxFields[i].rate, diskLinuxFields[i].unit, diskLinuxFields[i].desc)
			}
			if read_sectors != 0 && msec_read != 0 {
				Add(&md, metric+"time_per_read", read_sectors/msec_read, ts, metadata.Rate, metadata.MilliSecond, "")
			}
			if write_sectors != 0 && msec_write != 0 {
				Add(&md, metric+"time_per_write", write_sectors/msec_write, ts, metadata.Rate, metadata.MilliSecond, "")
			}
			if block_size != 0 {
				Add(&md, metric+"bytes", int64(write_sectors)*block_size, opentsdb.TagSet{"type": "write"}.Merge(ts), metadata.Counter, metadata.Bytes, "Total number of bytes written to disk.")
				Add(&md, metric+"bytes", int64(read_sectors)*block_size, opentsdb.TagSet{"type": "read"}.Merge(ts), metadata.Counter, metadata.Bytes, "Total number of bytes read to disk.")
				Add(&md, metric+"block_size", block_size, ts, metadata.Gauge, metadata.Bytes, "Sector size of the block device.")
			}
		} else if len(values) == 7 {
			for i, v := range values[3:] {
				Add(&md, metric+diskLinuxFieldsPart[i].key, v, ts, diskLinuxFieldsPart[i].rate, diskLinuxFieldsPart[i].unit, "")
			}
		} else {
			return fmt.Errorf("cannot parse")
		}
		return nil
	})
	return md, err
}

func examineMdadmVolume(volumeName string) (volumeDetail, error) {
	// command to get mdadm status
	tmout := 2 * time.Second
	// We don't use --test because it has failed us in the past.
	// Maybe we should use it sometime in the future
	output, err := util.Command(tmout, nil, "mdadm", "--detail", volumeName)
	if err != nil {
		return volumeDetail{}, err
	}
	detail := parseExamineMdadm(output)
	return detail, err
}

// keep only fileNames that are devices
func filterVolumes(volumes []string) []string {
	out := make([]string, 0, len(volumes))
	for _, vol := range volumes {
		finfo, err := os.Stat(vol)
		if err != nil { // if we can't stat, we won't monitor
			continue
		}
		if finfo.Mode()&os.ModeDevice != 0 {
			out = append(out, vol)
		}
	}
	return out
}

func checkMdadmLinux() (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint

	volumes, err := filepath.Glob("/dev/md*")
	if err != nil {
		return md, err
	}
	for _, volume := range filterVolumes(volumes) {
		detail, err := examineMdadmVolume(volume)
		if err != nil {
			slog.Errorf("mdadm: can't parse %s data, %s", volume, err)
			continue
		}
		addMdadmMetric(&md, volume, detail)
	}
	return md, nil
}

func c_dfstat_blocks_linux() (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	err := util.ReadCommand(func(line string) error {
		fields := strings.Fields(line)
		// TODO: support mount points with spaces in them. They mess up the field order
		// currently due to df's columnar output.
		if len(fields) != 7 || !IsDigit(fields[2]) {
			return nil
		}
		// /dev/mapper/vg0-usr ext4 13384816 9996920 2815784 79% /usr
		fs := fields[0]
		fsType := fields[1]
		spaceTotal := fields[2]
		spaceUsed := fields[3]
		spaceFree := fields[4]
		mount := fields[6]
		if isPseudoFS(fsType) {
			return nil
		}
		tags := opentsdb.TagSet{"mount": mount}
		os_tags := opentsdb.TagSet{"disk": mount}
		metric := "linux.disk.fs."
		ometric := "os.disk.fs."
		if removable_fs(fs) {
			metric += "rem."
			ometric += "rem."
		}
		Add(&md, metric+"space_total", spaceTotal, tags, metadata.Gauge, metadata.Bytes, osDiskTotalDesc)
		Add(&md, metric+"space_used", spaceUsed, tags, metadata.Gauge, metadata.Bytes, osDiskUsedDesc)
		Add(&md, metric+"space_free", spaceFree, tags, metadata.Gauge, metadata.Bytes, osDiskFreeDesc)
		Add(&md, ometric+"space_total", spaceTotal, os_tags, metadata.Gauge, metadata.Bytes, osDiskTotalDesc)
		Add(&md, ometric+"space_used", spaceUsed, os_tags, metadata.Gauge, metadata.Bytes, osDiskUsedDesc)
		Add(&md, ometric+"space_free", spaceFree, os_tags, metadata.Gauge, metadata.Bytes, osDiskFreeDesc)
		st, _ := strconv.ParseFloat(spaceTotal, 64)
		sf, _ := strconv.ParseFloat(spaceFree, 64)
		if st != 0 {
			Add(&md, osDiskPctFree, sf/st*100, os_tags, metadata.Gauge, metadata.Pct, osDiskPctFreeDesc)
		}
		return nil
	}, "df", "-lPT", "--block-size", "1")
	return md, err
}

func c_dfstat_inodes_linux() (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	err := util.ReadCommand(func(line string) error {
		fields := strings.Fields(line)
		if len(fields) != 7 || !IsDigit(fields[2]) {
			return nil
		}
		// /dev/mapper/vg0-usr ext4 851968 468711 383257 56% /usr
		fs := fields[0]
		fsType := fields[1]
		inodesTotal := fields[2]
		inodesUsed := fields[3]
		inodesFree := fields[4]
		mount := fields[6]
		if isPseudoFS(fsType) {
			return nil
		}
		tags := opentsdb.TagSet{"mount": mount}
		metric := "linux.disk.fs."
		if removable_fs(fs) {
			metric += "rem."
		}
		Add(&md, metric+"inodes_total", inodesTotal, tags, metadata.Gauge, metadata.Count, "")
		Add(&md, metric+"inodes_used", inodesUsed, tags, metadata.Gauge, metadata.Count, "")
		Add(&md, metric+"inodes_free", inodesFree, tags, metadata.Gauge, metadata.Count, "")
		return nil
	}, "df", "-liPT")
	return md, err
}
