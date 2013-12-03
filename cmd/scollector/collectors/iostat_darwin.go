package collectors

import (
	"strings"

	"github.com/StackExchange/tcollector/opentsdb"
)

func init() {
	collectors = append(collectors, c_iostat_darwin)
}

func c_iostat_darwin() opentsdb.MultiDataPoint {
	b, err := command("iostat", "-c2", "-w1")
	if err != nil {
		l.Println("iostat:", err)
		return nil
	}
	var md opentsdb.MultiDataPoint
	s := string(b)
	lines := strings.Split(s, "\n")
	if len(lines) < 4 {
		l.Println("iostat: bad return value")
		return nil
	}
	categories := strings.Fields(lines[0])
	values := strings.Fields(lines[3])
	i := 0
	for _, cat := range categories {
		if strings.HasPrefix(cat, "disk") {
			Add(&md, "iostat.disk.KBt", values[i], opentsdb.TagSet{"disk": cat})
			i++
			Add(&md, "iostat.disk.tps", values[i], opentsdb.TagSet{"disk": cat})
			i++
			Add(&md, "iostat.disk.MBs", values[i], opentsdb.TagSet{"disk": cat})
			i++
		} else if strings.HasPrefix(cat, "cpu") {
			Add(&md, "iostat.cpu.user", values[i], nil)
			i++
			Add(&md, "iostat.cpu.sys", values[i], nil)
			i++
			Add(&md, "iostat.cpu.idle", values[i], nil)
			i++
		} else if strings.HasPrefix(cat, "loadaverage") {
			Add(&md, "iostat.loadaverage.1m", values[i], nil)
			i++
			Add(&md, "iostat.loadaverage.5m", values[i], nil)
			i++
			Add(&md, "iostat.loadaverage.15m", values[i], nil)
			i++
		}
	}
	return md
}
