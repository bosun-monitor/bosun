package collectors

import (
	"strings"
	"time"

	"github.com/StackExchange/scollector/metadata"
	"github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/scollector/util"
)

func init() {
	const interval = time.Minute * 5
	collectors = append(collectors,
		&IntervalCollector{F: c_omreport_chassis, Interval: interval},
		&IntervalCollector{F: c_omreport_ps, Interval: interval},
		&IntervalCollector{F: c_omreport_ps_amps, Interval: interval},
		&IntervalCollector{F: c_omreport_ps_volts, Interval: interval},
		&IntervalCollector{F: c_omreport_storage_battery, Interval: interval},
		&IntervalCollector{F: c_omreport_storage_controller, Interval: interval},
		&IntervalCollector{F: c_omreport_storage_enclosure, Interval: interval},
		&IntervalCollector{F: c_omreport_storage_vdisk, Interval: interval},
		&IntervalCollector{F: c_omreport_system, Interval: interval},
	)
}

func c_omreport_chassis() opentsdb.MultiDataPoint {
	var md opentsdb.MultiDataPoint
	util.ReadCommand(func(line string) {
		fields := strings.Split(line, ";")
		if len(fields) != 2 || fields[0] == "SEVERITY" {
			return
		}
		sev := 0
		if fields[0] != "Ok" && fields[0] != "Non-Critical" {
			sev = 1
		}
		component := strings.Replace(fields[1], " ", "_", -1)
		Add(&md, "hw.chassis", sev, opentsdb.TagSet{"component": component}, metadata.Unknown, metadata.None, "")
	}, "omreport", "chassis", "-fmt", "ssv")
	return md
}

func c_omreport_system() opentsdb.MultiDataPoint {
	var md opentsdb.MultiDataPoint
	util.ReadCommand(func(line string) {
		fields := strings.Split(line, ";")
		if len(fields) != 2 || fields[0] == "SEVERITY" {
			return
		}
		sev := 0
		if fields[0] != "Ok" && fields[0] != "Non-Critical" {
			sev = 1
		}
		component := strings.Replace(fields[1], " ", "_", -1)
		Add(&md, "hw.system", sev, opentsdb.TagSet{"component": component}, metadata.Unknown, metadata.None, "")
	}, "omreport", "system", "-fmt", "ssv")
	return md
}

func c_omreport_storage_enclosure() opentsdb.MultiDataPoint {
	var md opentsdb.MultiDataPoint
	util.ReadCommand(func(line string) {
		fields := strings.Split(line, ";")
		if len(fields) < 3 || fields[0] == "ID" {
			return
		}
		sev := 0
		if fields[1] != "Ok" && fields[1] != "Non-Critical" {
			sev = 1
		}
		id := strings.Replace(fields[0], ":", "_", -1)
		Add(&md, "hw.storage.enclosure", sev, opentsdb.TagSet{"id": id}, metadata.Unknown, metadata.None, "")
	}, "omreport", "storage", "enclosure", "-fmt", "ssv")
	return md
}

func c_omreport_storage_vdisk() opentsdb.MultiDataPoint {
	var md opentsdb.MultiDataPoint
	util.ReadCommand(func(line string) {
		fields := strings.Split(line, ";")
		if len(fields) < 3 || fields[0] == "ID" {
			return
		}
		sev := 0
		if fields[1] != "Ok" && fields[1] != "Non-Critical" {
			sev = 1
		}
		id := strings.Replace(fields[0], ":", "_", -1)
		Add(&md, "hw.storage.vdisk", sev, opentsdb.TagSet{"id": id}, metadata.Unknown, metadata.None, "")
	}, "omreport", "storage", "vdisk", "-fmt", "ssv")
	return md
}

func c_omreport_ps() opentsdb.MultiDataPoint {
	var md opentsdb.MultiDataPoint
	util.ReadCommand(func(line string) {
		fields := strings.Split(line, ";")
		if len(fields) < 3 || fields[0] == "Index" {
			return
		}
		sev := 0
		if fields[1] != "Ok" {
			sev = 1
		}
		id := strings.Replace(fields[0], ":", "_", -1)
		Add(&md, "hw.ps", sev, opentsdb.TagSet{"id": id}, metadata.Unknown, metadata.None, "")
	}, "omreport", "chassis", "pwrsupplies", "-fmt", "ssv")
	return md
}

func c_omreport_ps_amps() opentsdb.MultiDataPoint {
	var md opentsdb.MultiDataPoint
	util.ReadCommand(func(line string) {
		fields := strings.Split(line, ";")
		if len(fields) != 2 || !strings.Contains(fields[0], "Current") {
			return
		}
		i_fields := strings.Split(fields[0], "Current")
		v_fields := strings.Fields(fields[1])
		if len(i_fields) < 2 && len(v_fields) < 2 {
			return
		}
		id := strings.Replace(i_fields[0], " ", "", -1)
		Add(&md, "hw.ps.current", v_fields[0], opentsdb.TagSet{"id": id}, metadata.Unknown, metadata.None, "")
	}, "omreport", "chassis", "pwrmonitoring", "-fmt", "ssv")
	return md
}

func c_omreport_ps_volts() opentsdb.MultiDataPoint {
	var md opentsdb.MultiDataPoint
	util.ReadCommand(func(line string) {
		fields := strings.Split(line, ";")
		if len(fields) != 8 || !strings.Contains(fields[2], "Voltage") || fields[3] == "[N/A]" {
			return
		}
		i_fields := strings.Split(fields[2], "Voltage")
		v_fields := strings.Fields(fields[3])
		if len(i_fields) < 2 && len(v_fields) < 2 {
			return
		}
		id := strings.Replace(i_fields[0], " ", "", -1)
		Add(&md, "hw.ps.volts", v_fields[0], opentsdb.TagSet{"id": id}, metadata.Unknown, metadata.None, "")
	}, "omreport", "chassis", "volts", "-fmt", "ssv")
	return md
}

func c_omreport_storage_battery() opentsdb.MultiDataPoint {
	var md opentsdb.MultiDataPoint
	util.ReadCommand(func(line string) {
		fields := strings.Split(line, ";")
		if len(fields) < 3 || fields[0] == "ID" {
			return
		}
		sev := 0
		if fields[1] != "Ok" && fields[1] != "Non-Critical" {
			sev = 1
		}
		id := strings.Replace(fields[0], ":", "_", -1)
		Add(&md, "hw.storage.battery", sev, opentsdb.TagSet{"id": id}, metadata.Unknown, metadata.None, "")
	}, "omreport", "storage", "battery", "-fmt", "ssv")
	return md
}

func c_omreport_storage_controller() opentsdb.MultiDataPoint {
	var md opentsdb.MultiDataPoint
	util.ReadCommand(func(line string) {
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
		Add(&md, "hw.storage.controller", sev, opentsdb.TagSet{"id": id}, metadata.Unknown, metadata.None, "")
	}, "omreport", "storage", "controller", "-fmt", "ssv")
	return md
}

// c_omreport_storage_pdisk is called from the controller func, since it needs the encapsulating id.
func c_omreport_storage_pdisk(id string, md *opentsdb.MultiDataPoint) {
	util.ReadCommand(func(line string) {
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
		Add(md, "hw.storage.pdisk", sev, opentsdb.TagSet{"id": id}, metadata.Unknown, metadata.None, "")
	}, "omreport", "storage", "pdisk", "controller="+id, "-fmt", "ssv")
}
