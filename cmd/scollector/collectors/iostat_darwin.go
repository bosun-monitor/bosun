package collectors

import (
	"strings"

	"github.com/StackExchange/tcollector/opentsdb"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_iostat_darwin})
}

func c_iostat_darwin() opentsdb.MultiDataPoint {
	var categories []string
	var md opentsdb.MultiDataPoint
	ln := 0
	i := 0
	readCommand(func(line string) {
		ln++
		if ln == 1 {
			categories = strings.Fields(line)
		}
		if ln < 4 {
			return
		}
		values := strings.Fields(line)
		for _, cat := range categories {
			if strings.HasPrefix(cat, "disk") {
				Add(&md, "darwin.disk.kilobytes_transfer", values[i], opentsdb.TagSet{"disk": cat})
				i++
				Add(&md, "darwin.disk.transactions", values[i], opentsdb.TagSet{"disk": cat})
				i++
				Add(&md, "darwin.disk.megabytes", values[i], opentsdb.TagSet{"disk": cat})
				i++
			} else if cat == "cpu" {
				Add(&md, "darwin.cpu.user", values[i], nil)
				i++
				Add(&md, "darwin.cpu.sys", values[i], nil)
				i++
				Add(&md, "darwin.cpu.idle", values[i], nil)
				i++
			} else if cat == "load" {
				Add(&md, "darwin.loadavg_1_min", values[i], nil)
				i++
				Add(&md, "darwin.loadavg_5_min", values[i], nil)
				i++
				Add(&md, "darwin.loadavg_15_min", values[i], nil)
				i++
			}
		}
	}, "iostat", "-c2", "-w1")
	if ln < 4 {
		l.Println("iostat: bad return value")
	}
	return md
}
