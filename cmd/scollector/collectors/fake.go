package collectors

import (
	"strconv"
	"time"

	"github.com/StackExchange/scollector/opentsdb"
)

func InitFake(fake int) {
	collectors = append(collectors, &IntervalCollector{
		F: func() opentsdb.MultiDataPoint {
			var md opentsdb.MultiDataPoint
			for i := 0; i < fake; i++ {
				Add(&md, "test.fake", i, opentsdb.TagSet{"i": strconv.Itoa(i)})
			}
			return md
		},
		Interval: time.Second,
		name:     "fake",
	})
}
