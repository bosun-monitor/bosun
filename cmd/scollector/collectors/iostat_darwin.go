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
				Add(&md, "iostat.disk.KBt", values[i], opentsdb.TagSet{"disk": cat})
				i++
				Add(&md, "iostat.disk.tps", values[i], opentsdb.TagSet{"disk": cat})
				i++
				Add(&md, "iostat.disk.MBs", values[i], opentsdb.TagSet{"disk": cat})
				i++
			} else if cat == "cpu" {
				Add(&md, "iostat.cpu.user", values[i], nil)
				i++
				Add(&md, "iostat.cpu.sys", values[i], nil)
				i++
				Add(&md, "iostat.cpu.idle", values[i], nil)
				i++
			} else if cat == "load" {
				Add(&md, "iostat.loadaverage.1m", values[i], nil)
				i++
				Add(&md, "iostat.loadaverage.5m", values[i], nil)
				i++
				Add(&md, "iostat.loadaverage.15m", values[i], nil)
				i++
			}
		}
	}, "iostat", "-c2", "-w1")
	if ln < 4 {
		l.Println("iostat: bad return value")
	}
	return md
}
