package collectors

import (
	"fmt"
	"strings"

	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"bosun.org/util"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: cIostatDarwin})
}

func cIostatDarwin() (opentsdb.MultiDataPoint, error) {
	var categories []string
	var md opentsdb.MultiDataPoint
	ln := 0
	i := 0
	util.ReadCommand(func(line string) error {
		ln++
		if ln == 1 {
			categories = strings.Fields(line)
		}
		if ln < 4 {
			return nil
		}
		values := strings.Fields(line)
		for _, cat := range categories {
			if i+3 > len(values) {
				break
			} else if strings.HasPrefix(cat, "disk") {
				Add(&md, "darwin.disk.kilobytes_transfer", values[i], opentsdb.TagSet{"disk": cat}, metadata.Unknown, metadata.None, "")
				i++
				Add(&md, "darwin.disk.transactions", values[i], opentsdb.TagSet{"disk": cat}, metadata.Unknown, metadata.None, "")
				i++
				Add(&md, "darwin.disk.megabytes", values[i], opentsdb.TagSet{"disk": cat}, metadata.Unknown, metadata.None, "")
				i++
			} else if cat == "cpu" {
				Add(&md, "darwin.cpu.user", values[i], nil, metadata.Gauge, metadata.Pct, descDarwinCPUUser)
				i++
				Add(&md, "darwin.cpu.sys", values[i], nil, metadata.Gauge, metadata.Pct, descDarwinCPUSys)
				i++
				Add(&md, "darwin.cpu.idle", values[i], nil, metadata.Gauge, metadata.Pct, descDarwinCPUIdle)
				i++
			} else if cat == "load" {
				Add(&md, "darwin.loadavg_1_min", values[i], nil, metadata.Unknown, metadata.None, "")
				i++
				Add(&md, "darwin.loadavg_5_min", values[i], nil, metadata.Unknown, metadata.None, "")
				i++
				Add(&md, "darwin.loadavg_15_min", values[i], nil, metadata.Unknown, metadata.None, "")
				i++
			}
		}
		return nil
	}, "iostat", "-c2", "-w1")
	if ln < 4 {
		return nil, fmt.Errorf("bad return value")
	}
	return md, nil
}

const (
	descDarwinCPUUser = "Percent of time in user mode."
	descDarwinCPUSys  = "Percent of time in system mode."
	descDarwinCPUIdle = "Percent of time in idle mode."
)
