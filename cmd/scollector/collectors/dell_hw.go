package collectors

import (
	"strings"
	"time"

	"github.com/StackExchange/scollector/opentsdb"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_omreport_chassis_linux, Interval: time.Minute * 5})
	collectors = append(collectors, &IntervalCollector{F: c_omreport_system_linux, Interval: time.Minute * 5})
	collectors = append(collectors, &IntervalCollector{F: c_omreport_storage_enclosure_linux, Interval: time.Minute * 5})
	collectors = append(collectors, &IntervalCollector{F: c_omreport_storage_vdisk_linux, Interval: time.Minute * 5})
	collectors = append(collectors, &IntervalCollector{F: c_omreport_storage_controller_linux, Interval: time.Minute * 5})

}

func c_omreport_chassis_linux() opentsdb.MultiDataPoint {
	var md opentsdb.MultiDataPoint
	readCommand(func(line string) {
		fields := strings.Split(line, ";")
		if len(fields) != 2 || fields[0] == "SEVERITY" {
			return
		}
		sev := 0
		if fields[0] != "Ok" {
			sev = 1
		}
		component := strings.Replace(fields[1], " ", "_", -1)
		Add(&md, "hw.chassis", sev, opentsdb.TagSet{"component": component})
	}, "/opt/dell/srvadmin/bin/omreport", "chassis", "-fmt", "ssv")
	return md
}

func c_omreport_system_linux() opentsdb.MultiDataPoint {
	var md opentsdb.MultiDataPoint
	readCommand(func(line string) {
		fields := strings.Split(line, ";")
		if len(fields) != 2 || fields[0] == "SEVERITY" {
			return
		}
		sev := 0
		if fields[0] != "Ok" {
			sev = 1
		}
		component := strings.Replace(fields[1], " ", "_", -1)
		Add(&md, "hw.system", sev, opentsdb.TagSet{"component": component})
	}, "/opt/dell/srvadmin/bin/omreport", "system", "-fmt", "ssv")
	return md
}

func c_omreport_storage_enclosure_linux() opentsdb.MultiDataPoint {
	var md opentsdb.MultiDataPoint
	readCommand(func(line string) {
		fields := strings.Split(line, ";")
		if len(fields) < 3 || fields[0] == "ID" {
			return
		}
		sev := 0
		if fields[1] != "Ok" {
			sev = 1
		}
		id := strings.Replace(fields[0], ":", "_", -1)
		Add(&md, "hw.storage.enclosure", sev, opentsdb.TagSet{"id": id})
	}, "/opt/dell/srvadmin/bin/omreport", "storage", "enclosure", "-fmt", "ssv")
	return md
}

func c_omreport_storage_vdisk_linux() opentsdb.MultiDataPoint {
	var md opentsdb.MultiDataPoint
	readCommand(func(line string) {
		fields := strings.Split(line, ";")
		if len(fields) < 3 || fields[0] == "ID" {
			return
		}
		sev := 0
		if fields[1] != "Ok" {
			sev = 1
		}
		//Need to find out what the various ID formats might be
		id := strings.Replace(fields[0], ":", "_", -1)
		Add(&md, "hw.storage.vdisk", sev, opentsdb.TagSet{"id": id})
	}, "/opt/dell/srvadmin/bin/omreport", "storage", "vdisk", "-fmt", "ssv")
	return md
}

func c_omreport_storage_controller_linux() opentsdb.MultiDataPoint {
	var md opentsdb.MultiDataPoint
	readCommand(func(line string) {
		fields := strings.Split(line, ";")
		if len(fields) < 3 || fields[0] == "ID" {
			return
		}
		sev := 0
		if fields[1] != "Ok" {
			sev = 1
		}
		//Need to find out what the various ID formats might be
		id := strings.Replace(fields[0], ":", "_", -1)
		Add(&md, "hw.storage.controller", sev, opentsdb.TagSet{"id": id})
	}, "/opt/dell/srvadmin/bin/omreport", "storage", "controller", "-fmt", "ssv")
	return md
}
