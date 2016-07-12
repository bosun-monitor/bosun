package collectors

// things that are not OS specific.
// ie: can compile and execute anywhere

import (
	"bufio"
	"io"
	"strconv"
	"strings"

	"bosun.org/metadata"
	"bosun.org/opentsdb"
)

// Check mdadm raid arrays on linux
// linux.disk.mdadm.state values:

type mdadmState int

const (
	mdadmUnknown  mdadmState = 0
	mdadmNormal              = 1 // active or clean state
	mdadmFailed              = 2 // raid is failed
	mdadmDegraded            = 3 // raid is degraded
)

const (
	mdadmDesc = "raid 0: unknown, 1: normal, 2: failed, 3: degraded"
	syncDesc  = "percent of spindles synchronization. 100% is fully synced"
	spinDesc  = "spin 0: failed, 1: active, 2: spare"
)

type spinState int

// Check individual spindle disks in the array
const (
	spinFailed spinState = 0
	spinActive           = 1
	spinSpare            = 2
)

type volumeDetail struct {
	syncProgress  float32 // progress: between 0..100%
	state         mdadmState
	failedSpindle []string
	activeSpindle []string
	spareSpindle  []string
}

func getResync(l string) (progress float32, gotit bool) {
	prefix := "Rebuild Status : "
	if !strings.HasPrefix(l, prefix) {
		return 0, false
	}
	l = strings.TrimPrefix(l, prefix)
	pcentIdx := strings.Index(l, "%")
	if pcentIdx == -1 {
		return 0, false
	}

	f, err := strconv.ParseFloat(l[:pcentIdx], 32)
	return float32(f), err == nil
}

func getSpindle(l string) (dev string, gotit bool) {
	fields := strings.Split(l, " ")
	size := len(fields)
	if strings.Contains(fields[size-1], "/dev/sd") {
		return fields[size-1], true
	}
	return "", false
}

func getState(l string) (state mdadmState, gotit bool) {
	if !strings.HasPrefix(l, "State : ") {
		return mdadmUnknown, false
	}
	if strings.Contains(l, ", FAILED") {
		return mdadmFailed, true
	}
	if strings.Contains(l, ", degraded") {
		return mdadmDegraded, true
	}
	if strings.Contains(l, "clean") || strings.Contains(l, "active") {
		return mdadmNormal, true
	}
	return mdadmUnknown, false
}

func parseExamineMdadm(examine io.Reader) (detail volumeDetail) {
	scanner := bufio.NewScanner(examine)
	// if there is no progress spotted, assume the disks are in sync
	detail.syncProgress = 100.0
	for scanner.Scan() {
		l := scanner.Text()
		l = strings.Trim(l, " \t")

		// extract resync status
		if progress, ok := getResync(l); ok {
			detail.syncProgress = progress
		}

		// extract spindles
		if dev, ok := getSpindle(l); ok {
			if strings.Contains(l, "active sync") {
				detail.activeSpindle = append(detail.activeSpindle, dev)
			} else if strings.Contains(l, "spare") {
				detail.spareSpindle = append(detail.spareSpindle, dev)
			} else { // if we don't know, assume it's failed
				detail.failedSpindle = append(detail.failedSpindle, dev)
			}
		}

		// filter State
		if state, ok := getState(l); ok {
			detail.state = state
			// consider failed arrays as 0% resynced
			if state != mdadmNormal && state != mdadmDegraded {
				detail.syncProgress = 0.0
			}
		}
	}
	return detail
}

func addMetricSpindle(md *opentsdb.MultiDataPoint, names []string, status spinState, volume string) {
	metric := "linux.disk.mdadm.spindle"
	for _, name := range names {
		tags := opentsdb.TagSet{
			"volume":  volume,
			"spindle": name,
		}
		Add(md, metric, int(status), tags, metadata.Gauge, metadata.StatusCode, spinDesc)
	}
}

func addMdadmMetric(md *opentsdb.MultiDataPoint, volume string, detail volumeDetail) {
	tags := opentsdb.TagSet{"volume": volume}
	metric := "linux.disk.mdadm.state"
	Add(md, metric, detail.state, tags, metadata.Gauge, metadata.StatusCode, mdadmDesc)

	metric = "linux.disk.mdadm.sync"
	Add(md, metric, detail.syncProgress, tags, metadata.Gauge, metadata.Pct, syncDesc)

	addMetricSpindle(md, detail.failedSpindle, spinFailed, volume)
	addMetricSpindle(md, detail.activeSpindle, spinActive, volume)
	addMetricSpindle(md, detail.spareSpindle, spinSpare, volume)
}
