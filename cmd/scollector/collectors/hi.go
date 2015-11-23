package collectors

import (
	"bosun.org/metadata"
	"bosun.org/opentsdb"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_scollector_hi})
}

const (
	hiDesc = "Scollector sends a 1 every DefaultFreq. This is so you can alert on scollector being down."
)

func c_scollector_hi() (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	Add(&md, "scollector.hi", 1, nil, metadata.Gauge, metadata.Ok, hiDesc)
	return md, nil
}
