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
	readOmreport(func(fields []string) {
		if len(fields) != 2 || fields[0] == "SEVERITY" {
			return
		}
		component := strings.Replace(fields[1], " ", "_", -1)
		Add(&md, "hw.chassis", severity(fields[0]), opentsdb.TagSet{"component": component}, metadata.Unknown, metadata.None, "")
	}, "chassis")
	return md
}

func c_omreport_system() opentsdb.MultiDataPoint {
	var md opentsdb.MultiDataPoint
	readOmreport(func(fields []string) {
		if len(fields) != 2 || fields[0] == "SEVERITY" {
			return
		}
		component := strings.Replace(fields[1], " ", "_", -1)
		Add(&md, "hw.system", severity(fields[1]), opentsdb.TagSet{"component": component}, metadata.Unknown, metadata.None, "")
	}, "system")
	return md
}

func c_omreport_storage_enclosure() opentsdb.MultiDataPoint {
	var md opentsdb.MultiDataPoint
	readOmreport(func(fields []string) {
		if len(fields) < 3 || fields[0] == "ID" {
			return
		}
		id := strings.Replace(fields[0], ":", "_", -1)
		Add(&md, "hw.storage.enclosure", severity(fields[1]), opentsdb.TagSet{"id": id}, metadata.Unknown, metadata.None, "")
	}, "storage", "enclosure")
	return md
}

func c_omreport_storage_vdisk() opentsdb.MultiDataPoint {
	var md opentsdb.MultiDataPoint
	readOmreport(func(fields []string) {
		if len(fields) < 3 || fields[0] == "ID" {
			return
		}
		id := strings.Replace(fields[0], ":", "_", -1)
		Add(&md, "hw.storage.vdisk", severity(fields[1]), opentsdb.TagSet{"id": id}, metadata.Unknown, metadata.None, "")
	}, "storage", "vdisk")
	return md
}

func c_omreport_ps() opentsdb.MultiDataPoint {
	var md opentsdb.MultiDataPoint
	readOmreport(func(fields []string) {
		if len(fields) < 3 || fields[0] == "Index" {
			return
		}
		id := strings.Replace(fields[0], ":", "_", -1)
		Add(&md, "hw.ps", severity(fields[1]), opentsdb.TagSet{"id": id}, metadata.Unknown, metadata.None, "")
	}, "chassis", "pwrsupplies")
	return md
}

func c_omreport_ps_amps() opentsdb.MultiDataPoint {
	var md opentsdb.MultiDataPoint
	readOmreport(func(fields []string) {
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
	}, "chassis", "pwrmonitoring")
	return md
}

func c_omreport_ps_volts() opentsdb.MultiDataPoint {
	var md opentsdb.MultiDataPoint
	readOmreport(func(fields []string) {
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
	}, "chassis", "volts")
	return md
}

func c_omreport_storage_battery() opentsdb.MultiDataPoint {
	var md opentsdb.MultiDataPoint
	readOmreport(func(fields []string) {
		if len(fields) < 3 || fields[0] == "ID" {
			return
		}
		id := strings.Replace(fields[0], ":", "_", -1)
		Add(&md, "hw.storage.battery", severity(fields[1]), opentsdb.TagSet{"id": id}, metadata.Unknown, metadata.None, "")
	}, "storage", "battery")
	return md
}

func c_omreport_storage_controller() opentsdb.MultiDataPoint {
	var md opentsdb.MultiDataPoint
	readOmreport(func(fields []string) {
		if len(fields) < 3 || fields[0] == "ID" {
			return
		}
		c_omreport_storage_pdisk(fields[0], &md)
		id := strings.Replace(fields[0], ":", "_", -1)
		Add(&md, "hw.storage.controller", severity(fields[1]), opentsdb.TagSet{"id": id}, metadata.Unknown, metadata.None, "")
	}, "storage", "controller")
	return md
}

// c_omreport_storage_pdisk is called from the controller func, since it needs the encapsulating id.
func c_omreport_storage_pdisk(id string, md *opentsdb.MultiDataPoint) {
	readOmreport(func(fields []string) {
		if len(fields) < 3 || fields[0] == "ID" {
			return
		}
		//Need to find out what the various ID formats might be
		id := strings.Replace(fields[0], ":", "_", -1)
		Add(md, "hw.storage.pdisk", severity(fields[1]), opentsdb.TagSet{"id": id}, metadata.Unknown, metadata.None, "")
	}, "storage", "pdisk", "controller="+id)
}


// severity returns 0 if s is not "Ok" or "Non-Critical", else 1.
func severity(s string) int {
	if s != "Ok" && s != "Non-Critical" {
		return 1
	}
	return 0
}

func readOmreport(f func([]string), args ...string) {
	args = append(args, "-fmt", "ssv")
	util.ReadCommand(func(line string) {
		f(strings.Split(line, ";"))
	}, "omreport", args...)
}
