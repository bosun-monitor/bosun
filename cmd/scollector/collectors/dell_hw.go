package collectors

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"bosun.org/slog"
	"bosun.org/util"
)

func init() {
	const interval = time.Minute * 5
	collectors = append(collectors,
		&IntervalCollector{F: c_omreport_chassis, Interval: interval},
		&IntervalCollector{F: c_omreport_fans, Interval: interval},
		&IntervalCollector{F: c_omreport_memory, Interval: interval},
		&IntervalCollector{F: c_omreport_processors, Interval: interval},
		&IntervalCollector{F: c_omreport_ps, Interval: interval},
		&IntervalCollector{F: c_omreport_ps_amps_sysboard_pwr, Interval: interval},
		&IntervalCollector{F: c_omreport_storage_battery, Interval: interval},
		&IntervalCollector{F: c_omreport_storage_controller, Interval: interval},
		&IntervalCollector{F: c_omreport_storage_enclosure, Interval: interval},
		&IntervalCollector{F: c_omreport_storage_vdisk, Interval: interval},
		&IntervalCollector{F: c_omreport_system, Interval: interval},
		&IntervalCollector{F: c_omreport_temps, Interval: interval},
		&IntervalCollector{F: c_omreport_volts, Interval: interval},
	)
}

func c_omreport_chassis() (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	readOmreport(func(fields []string) {
		if len(fields) != 2 || fields[0] == "SEVERITY" {
			return
		}
		component := strings.Replace(fields[1], " ", "_", -1)
		Add(&md, "hw.chassis", severity(fields[0]), opentsdb.TagSet{"component": component}, metadata.Gauge, metadata.Ok, descDellHWChassis)
	}, "chassis")
	return md, nil
}

func c_omreport_system() (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	readOmreport(func(fields []string) {
		if len(fields) != 2 || fields[0] == "SEVERITY" {
			return
		}
		component := strings.Replace(fields[1], " ", "_", -1)
		Add(&md, "hw.system", severity(fields[0]), opentsdb.TagSet{"component": component}, metadata.Gauge, metadata.Ok, descDellHWSystem)
	}, "system")
	return md, nil
}

func c_omreport_storage_enclosure() (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	readOmreport(func(fields []string) {
		if len(fields) < 3 || fields[0] == "ID" {
			return
		}
		id := strings.Replace(fields[0], ":", "_", -1)
		Add(&md, "hw.storage.enclosure", severity(fields[1]), opentsdb.TagSet{"id": id}, metadata.Gauge, metadata.Ok, descDellHWStorageEnc)
	}, "storage", "enclosure")
	return md, nil
}

func c_omreport_storage_vdisk() (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	readOmreport(func(fields []string) {
		if len(fields) < 3 || fields[0] == "ID" {
			return
		}
		id := strings.Replace(fields[0], ":", "_", -1)
		Add(&md, "hw.storage.vdisk", severity(fields[1]), opentsdb.TagSet{"id": id}, metadata.Gauge, metadata.Ok, descDellHWVDisk)
	}, "storage", "vdisk")
	return md, nil
}

func c_omreport_ps() (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	readOmreport(func(fields []string) {
		if len(fields) < 3 || fields[0] == "Index" {
			return
		}
		id := strings.Replace(fields[0], ":", "_", -1)
		Add(&md, "hw.ps", severity(fields[1]), opentsdb.TagSet{"id": id}, metadata.Gauge, metadata.Ok, descDellHWPS)
	}, "chassis", "pwrsupplies")
	return md, nil
}

func c_omreport_ps_amps_sysboard_pwr() (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	readOmreport(func(fields []string) {
		if len(fields) == 2 && strings.Contains(fields[0], "Current") {
			i_fields := strings.Split(fields[0], "Current")
			v_fields := strings.Fields(fields[1])
			if len(i_fields) < 2 && len(v_fields) < 2 {
				return
			}
			id := strings.Replace(i_fields[0], " ", "", -1)
			Add(&md, "hw.chassis.current.reading", v_fields[0], opentsdb.TagSet{"id": id}, metadata.Gauge, metadata.A, descDellHWCurrent)
		} else if len(fields) == 6 && (fields[2] == "System Board Pwr Consumption" || fields[2] == "System Board System Level") {
			v_fields := strings.Fields(fields[3])
			warn_fields := strings.Fields(fields[4])
			fail_fields := strings.Fields(fields[5])
			if len(v_fields) < 2 || len(warn_fields) < 2 || len(fail_fields) < 2 {
				return
			}
			Add(&md, "hw.chassis.power.reading", v_fields[0], nil, metadata.Gauge, metadata.Watt, descDellHWPower)
			Add(&md, "hw.chassis.power.warn_level", warn_fields[0], nil, metadata.Gauge, metadata.Watt, descDellHWPowerThreshold)
			Add(&md, "hw.chassis.power.fail_level", fail_fields[0], nil, metadata.Gauge, metadata.Watt, descDellHWPowerThreshold)
		}
	}, "chassis", "pwrmonitoring")
	return md, nil
}

func c_omreport_storage_battery() (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	readOmreport(func(fields []string) {
		if len(fields) < 3 || fields[0] == "ID" {
			return
		}
		id := strings.Replace(fields[0], ":", "_", -1)
		Add(&md, "hw.storage.battery", severity(fields[1]), opentsdb.TagSet{"id": id}, metadata.Gauge, metadata.Ok, descDellHWStorageBattery)
	}, "storage", "battery")
	return md, nil
}

func c_omreport_storage_controller() (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	readOmreport(func(fields []string) {
		if len(fields) < 3 || fields[0] == "ID" {
			return
		}
		c_omreport_storage_pdisk(fields[0], &md)
		id := strings.Replace(fields[0], ":", "_", -1)
		Add(&md, "hw.storage.controller", severity(fields[1]), opentsdb.TagSet{"id": id}, metadata.Gauge, metadata.Ok, descDellHWStorageCtl)
	}, "storage", "controller")
	return md, nil
}

// c_omreport_storage_pdisk is called from the controller func, since it needs the encapsulating id.
func c_omreport_storage_pdisk(id string, md *opentsdb.MultiDataPoint) {
	readOmreport(func(fields []string) {
		if len(fields) < 3 || fields[0] == "ID" {
			return
		}
		//Need to find out what the various ID formats might be
		id := strings.Replace(fields[0], ":", "_", -1)
		ts := opentsdb.TagSet{"id": id}
		Add(md, "hw.storage.pdisk", severity(fields[1]), ts, metadata.Gauge, metadata.Ok, descDellHWPDisk)
		if len(fields) < 32 {
			return
		}
		dm := &metadata.HWDiskMeta{}
		if fields[2] != "" {
			dm.Name = fields[2]
		}
		if fields[6] != "" {
			dm.Media = fields[6]
		}
		if fields[19] != "" {
			dm.Capacity = fields[19]
		}
		if fields[23] != "" {
			dm.VendorId = fields[23]
		}
		if fields[24] != "" {
			dm.ProductId = fields[24]
		}
		if fields[25] != "" {
			dm.Serial = fields[25]
		}
		if fields[26] != "" {
			dm.Part = fields[26]
		}
		if fields[27] != "" {
			dm.NegotatiedSpeed = fields[27]
		}
		if fields[28] != "" {
			dm.CapableSpeed = fields[28]
		}
		if fields[31] != "" {
			dm.SectorSize = fields[31]

		}
		if j, err := json.Marshal(&dm); err == nil {
			metadata.AddMeta("", ts, "physicalDiskMeta", string(j), true)
		} else {
			slog.Error(err)
		}

	}, "storage", "pdisk", "controller="+id)
}

func c_omreport_processors() (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	readOmreport(func(fields []string) {
		if len(fields) != 8 {
			return
		}
		if _, err := strconv.Atoi(fields[0]); err != nil {
			return
		}
		ts := opentsdb.TagSet{"name": replace(fields[2])}
		Add(&md, "hw.chassis.processor", severity(fields[1]), ts, metadata.Gauge, metadata.Ok, descDellHWCPU)
		metadata.AddMeta("", ts, "processor", clean(fields[3], fields[4]), true)
	}, "chassis", "processors")
	return md, nil
}

func c_omreport_fans() (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	readOmreport(func(fields []string) {
		if len(fields) != 8 {
			return
		}
		if _, err := strconv.Atoi(fields[0]); err != nil {
			return
		}
		ts := opentsdb.TagSet{"name": replace(fields[2])}
		Add(&md, "hw.chassis.fan", severity(fields[1]), ts, metadata.Gauge, metadata.Ok, descDellHWFan)
		fs := strings.Fields(fields[3])
		if len(fs) == 2 && fs[1] == "RPM" {
			i, err := strconv.Atoi(fs[0])
			if err == nil {
				Add(&md, "hw.chassis.fan.reading", i, ts, metadata.Gauge, metadata.RPM, descDellHWFanSpeed)
			}
		}
	}, "chassis", "fans")
	return md, nil
}

func c_omreport_memory() (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	readOmreport(func(fields []string) {
		if len(fields) != 5 {
			return
		}
		if _, err := strconv.Atoi(fields[0]); err != nil {
			return
		}
		ts := opentsdb.TagSet{"name": replace(fields[2])}
		Add(&md, "hw.chassis.memory", severity(fields[1]), ts, metadata.Gauge, metadata.Ok, descDellHWMemory)
		metadata.AddMeta("", ts, "memory", clean(fields[4]), true)
	}, "chassis", "memory")
	return md, nil
}

func c_omreport_temps() (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	readOmreport(func(fields []string) {
		if len(fields) != 8 {
			return
		}
		if _, err := strconv.Atoi(fields[0]); err != nil {
			return
		}
		ts := opentsdb.TagSet{"name": replace(fields[2])}
		Add(&md, "hw.chassis.temps", severity(fields[1]), ts, metadata.Gauge, metadata.Ok, descDellHWTemp)
		fs := strings.Fields(fields[3])
		if len(fs) == 2 && fs[1] == "C" {
			i, err := strconv.ParseFloat(fs[0], 64)
			if err == nil {
				Add(&md, "hw.chassis.temps.reading", i, ts, metadata.Gauge, metadata.C, descDellHWTempReadings)
			}
		}
	}, "chassis", "temps")
	return md, nil
}

func c_omreport_volts() (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	readOmreport(func(fields []string) {
		if len(fields) != 8 {
			return
		}
		if _, err := strconv.Atoi(fields[0]); err != nil {
			return
		}
		ts := opentsdb.TagSet{"name": replace(fields[2])}
		Add(&md, "hw.chassis.volts", severity(fields[1]), ts, metadata.Gauge, metadata.Ok, descDellHWVolt)
		if i, err := extract(fields[3], "V"); err == nil {
			Add(&md, "hw.chassis.volts.reading", i, ts, metadata.Gauge, metadata.V, descDellHWVoltReadings)
		}
	}, "chassis", "volts")
	return md, nil
}

// extract tries to return a parsed number from s with given suffix. A space may
// be present between number ond suffix.
func extract(s, suffix string) (float64, error) {
	if !strings.HasSuffix(s, suffix) {
		return 0, fmt.Errorf("extract: suffix not found")
	}
	s = s[:len(s)-len(suffix)]
	return strconv.ParseFloat(strings.TrimSpace(s), 64)
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
	_ = util.ReadCommand(func(line string) error {
		sp := strings.Split(line, ";")
		for i, s := range sp {
			sp[i] = clean(s)
		}
		f(sp)
		return nil
	}, "omreport", args...)
}

// clean concatenates arguments with a space and removes extra whitespace.
func clean(ss ...string) string {
	v := strings.Join(ss, " ")
	fs := strings.Fields(v)
	return strings.Join(fs, " ")
}

func replace(name string) string {
	r, _ := opentsdb.Replace(name, "_")
	return r
}

const (
	descDellHWChassis        = "Overall status of chassis components."
	descDellHWSystem         = "Overall status of system components."
	descDellHWStorageEnc     = "Overall status of storage enclosures."
	descDellHWVDisk          = "Overall status of virtual disks."
	descDellHWPS             = "Overall status of power supplies."
	descDellHWCurrent        = "Amps used per power supply."
	descDellHWPower          = "System board power usage."
	descDellHWPowerThreshold = "The warning and failure levels set on the device for system board power usage."
	descDellHWStorageBattery = "Status of storage controller backup batteries."
	descDellHWStorageCtl     = "Overall status of storage controllers."
	descDellHWPDisk          = "Overall status of physical disks."
	descDellHWCPU            = "Overall status of CPUs."
	descDellHWFan            = "Overall status of system fans."
	descDellHWFanSpeed       = "System fan speed."
	descDellHWMemory         = "System RAM DIMM status."
	descDellHWTemp           = "Overall status of system temperature readings."
	descDellHWTempReadings   = "System temperature readings."
	descDellHWVolt           = "Overall status of power supply volt readings."
	descDellHWVoltReadings   = "Volts used per power supply."
)
