package collectors

import (
	"strings"
	"time"

	"github.com/StackExchange/scollector/opentsdb"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_omreport_chassis, Interval: time.Minute * 5})
	collectors = append(collectors, &IntervalCollector{F: c_omreport_system, Interval: time.Minute * 5})
	collectors = append(collectors, &IntervalCollector{F: c_omreport_storage_enclosure, Interval: time.Minute * 5})
	collectors = append(collectors, &IntervalCollector{F: c_omreport_storage_vdisk, Interval: time.Minute * 5})
	collectors = append(collectors, &IntervalCollector{F: c_omreport_storage_controller, Interval: time.Minute * 5})
	collectors = append(collectors, &IntervalCollector{F: c_omreport_storage_battery, Interval: time.Minute * 5})
	collectors = append(collectors, &IntervalCollector{F: c_omreport_ps, Interval: time.Minute * 5})
	collectors = append(collectors, &IntervalCollector{F: c_omreport_ps_amps, Interval: time.Minute * 5})
	collectors = append(collectors, &IntervalCollector{F: c_omreport_ps_volts, Interval: time.Minute * 5})

}

func c_omreport_chassis() opentsdb.MultiDataPoint {
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
	}, "omreport", "chassis", "-fmt", "ssv")
	return md
}

func c_omreport_system() opentsdb.MultiDataPoint {
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
	}, "omreport", "system", "-fmt", "ssv")
	return md
}

func c_omreport_storage_enclosure() opentsdb.MultiDataPoint {
	var md opentsdb.MultiDataPoint
	readCommand(func(line string) {
		fields := strings.Split(line, ";")
		if len(fields) < 3 || fields[0] == "ID" {
			return
		}
		sev := 0
		if fields[1] != "Ok" && fields[1] != "Non-Critical" {
			sev = 1
		}
		id := strings.Replace(fields[0], ":", "_", -1)
		Add(&md, "hw.storage.enclosure", sev, opentsdb.TagSet{"id": id})
	}, "omreport", "storage", "enclosure", "-fmt", "ssv")
	return md
}

func c_omreport_storage_vdisk() opentsdb.MultiDataPoint {
	var md opentsdb.MultiDataPoint
	readCommand(func(line string) {
		fields := strings.Split(line, ";")
		if len(fields) < 3 || fields[0] == "ID" {
			return
		}
		sev := 0
		if fields[1] != "Ok" && fields[1] != "Non-Critical" {
			sev = 1
		}
		id := strings.Replace(fields[0], ":", "_", -1)
		Add(&md, "hw.storage.vdisk", sev, opentsdb.TagSet{"id": id})
	}, "omreport", "storage", "vdisk", "-fmt", "ssv")
	return md
}

func c_omreport_ps() opentsdb.MultiDataPoint {
	var md opentsdb.MultiDataPoint
	readCommand(func(line string) {
		fields := strings.Split(line, ";")
		if len(fields) < 3 || fields[0] == "Index" {
			return
		}
		sev := 0
		if fields[1] != "Ok" {
			sev = 1
		}
		id := strings.Replace(fields[0], ":", "_", -1)
		Add(&md, "hw.ps", sev, opentsdb.TagSet{"id": id})
	}, "omreport", "chassis", "pwrsupplies", "-fmt", "ssv")
	return md
}

func c_omreport_ps_amps() opentsdb.MultiDataPoint {
	var md opentsdb.MultiDataPoint
	readCommand(func(line string) {
		fields := strings.Split(line, ";")
		if len(fields) != 2 || !strings.Contains(fields[0], "Current") {
			return
		}
		i_fields := strings.Fields(fields[0])
		v_fields := strings.Fields(fields[1])
		if len(i_fields) < 2 && len(v_fields) < 2 {
			return
		}
		Add(&md, "hw.ps.current", v_fields[0], opentsdb.TagSet{"id": i_fields[0]})
	}, "omreport", "chassis", "pwrmonitoring", "-fmt", "ssv")
	return md
}

func c_omreport_ps_volts() opentsdb.MultiDataPoint {
	var md opentsdb.MultiDataPoint
	readCommand(func(line string) {
		fields := strings.Split(line, ";")
		if len(fields) != 8 || !strings.Contains(fields[2], "Voltage") || fields[3] == "[N/A]" {
			return
		}
		i_fields := strings.Fields(fields[2])
		v_fields := strings.Fields(fields[3])
		if len(i_fields) < 2 && len(v_fields) < 2 {
			return
		}
		Add(&md, "hw.ps.volts", v_fields[0], opentsdb.TagSet{"id": i_fields[0]})
	}, "omreport", "chassis", "volts", "-fmt", "ssv")
	return md
}

func c_omreport_storage_battery() opentsdb.MultiDataPoint {
	var md opentsdb.MultiDataPoint
	readCommand(func(line string) {
		fields := strings.Split(line, ";")
		if len(fields) < 3 || fields[0] == "ID" {
			return
		}
		sev := 0
		if fields[1] != "Ok" && fields[1] != "Non-Critical" {
			sev = 1
		}
		id := strings.Replace(fields[0], ":", "_", -1)
		Add(&md, "hw.storage.battery", sev, opentsdb.TagSet{"id": id})
	}, "omreport", "storage", "battery", "-fmt", "ssv")
	return md
}

func c_omreport_storage_controller() opentsdb.MultiDataPoint {
	var md opentsdb.MultiDataPoint
	readCommand(func(line string) {
		fields := strings.Split(line, ";")
		if len(fields) < 3 || fields[0] == "ID" {
			return
		}
		sev := 0
		if fields[1] != "Ok" && fields[1] != "Non-Critical" {
			sev = 1
		}
		c_omreport_storage_pdisk(fields[0], &md)
		id := strings.Replace(fields[0], ":", "_", -1)
		Add(&md, "hw.storage.controller", sev, opentsdb.TagSet{"id": id})
	}, "omreport", "storage", "controller", "-fmt", "ssv")
	return md
}

// The following is called from controller, since it needs the encapsulating id
func c_omreport_storage_pdisk(id string, md *opentsdb.MultiDataPoint) {
	readCommand(func(line string) {
		fields := strings.Split(line, ";")
		if len(fields) < 3 || fields[0] == "ID" {
			return
		}
		sev := 0
		if fields[1] != "Ok" && fields[1] != "Non-Critical" {
			sev = 1
		}
		//Need to find out what the various ID formats might be
		id := strings.Replace(fields[0], ":", "_", -1)
		Add(md, "hw.storage.pdisk", sev, opentsdb.TagSet{"id": id})
	}, "omreport", "storage", "pdisk", "controller="+id, "-fmt", "ssv")
}
