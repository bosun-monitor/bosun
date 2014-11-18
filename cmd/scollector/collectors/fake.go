package collectors

import (
	"strconv"
	"time"

	"github.com/bosun-monitor/scollector/_third_party/github.com/bosun-monitor/metadata"
	"github.com/bosun-monitor/scollector/_third_party/github.com/bosun-monitor/opentsdb"
)

func InitFake(fake int) {
	collectors = append(collectors, &IntervalCollector{
		F: func() (opentsdb.MultiDataPoint, error) {
			var md opentsdb.MultiDataPoint
			for i := 0; i < fake; i++ {
				Add(&md, "test.fake", i, opentsdb.TagSet{"i": strconv.Itoa(i)}, metadata.Unknown, metadata.None, "")
			}
			return md, nil
		},
		Interval: time.Second,
		name:     "fake",
	})
}
