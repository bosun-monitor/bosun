package collectors

import (
	"strings"
	"time"

	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"bosun.org/slog"
)

var (
	tztQuery string // This will be created by wmiInitNamespace() by using reflection over the struct that's been passed
	rootWMI  = "root\\WMI"
)

func init() {
	c_tzt := &IntervalCollector{
		F:        c_thermalzonetemperature,
		Interval: time.Second * 60,
	}
	c_tzt.init = wmiInitNamespace(c_tzt, func() interface{} { return &[]MSAcpi_ThermalZoneTemperature{} }, "", &tztQuery, rootWMI)
	collectors = append(collectors, c_tzt)
}

func kelvintens_to_celsius(ktens uint64) float32 {
	celsius := (float32(ktens) - 2731.5) / 10
	return celsius
}

func c_thermalzonetemperature() (opentsdb.MultiDataPoint, error) {
	var dst []MSAcpi_ThermalZoneTemperature
	err := queryWmiNamespace(tztQuery, &dst, rootWMI)
	if err != nil {
		return nil, slog.Wrap(err)
	}
	var md opentsdb.MultiDataPoint
	for _, v := range dst {
		instance := strings.Split(v.InstanceName, "\\")
		tzone := instance[len(instance)-1]
		Add(&md, "win.hw.temps.current", kelvintens_to_celsius(v.CurrentTemperature), opentsdb.TagSet{"thermalzone": tzone}, metadata.C, metadata.Gauge, descCurrentTemperature)
		Add(&md, "win.hw.temps.crittrippoint", kelvintens_to_celsius(v.CriticalTripPoint), opentsdb.TagSet{"thermalzone": tzone}, metadata.C, metadata.Gauge, descCriticalTripPoint)
	}
	return md, nil
}

const (
	descCurrentTemperature = "Motherboard thermal zone current temperature: Celsius"
	descCriticalTripPoint  = "Motherboard thermal zone critical temperature threshold: Celsius"
)

type MSAcpi_ThermalZoneTemperature struct {
	InstanceName       string
	CurrentTemperature uint64
	CriticalTripPoint  uint64
}
