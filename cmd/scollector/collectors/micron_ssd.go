package collectors

import (
	"encoding/json"
	"os/exec"

	"bosun.org/metadata"
	"bosun.org/opentsdb"
)

func init() {
	c_micron := &IntervalCollector{
		F: c_dns_windows,
		Enable: func() bool {
			exe, err := exec.LookPath("msecli")
			if err != nil || exe == "" {
				return false
			}
			return true
		},
	}
	collectors = append(collectors, c_micron)
}

func c_micron_ssds() (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	cmd := exec.Command("msecli", "-L", "-J")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}
	var data = &struct {
		Drives []struct {
			DeviceName      string `json:"deviceName"`
			SerialNumber    string `json:"serialNumber"`
			ModelNumber     string `json:"modelNumber"`
			FirmwareVersion string `json:"firmwareVersion"`
			IsMicron        bool   `json:"isMicron"`
			DriverInfo      struct {
				Version string `json:"driverVersion"`
			} `json:"driverInfo"`
			SmartData struct {
				CurrentTemperature  json.Number `json:"currentTemperature"`
				AvailableSpareSpace json.Number `json:"availableSpareSpace"`
				PercentLifeUsed     json.Number `json:"percentLifeUsed"`
				DataUnitsRead       json.Number `json:"dataUnitsRead"`
				DataUnitsWritten    json.Number `json:"dataUnitsWritten"`
				PowerCycleCount     json.Number `json:"powerCycleCount"`
				PowerOnHours        json.Number `json:"powerOnHours"`
				UnsafeShutdowns     json.Number `json:"unsafeShutdowns"`
				MediaErrors         json.Number `json:"mediaErrors"`
				ErrorLogCount       json.Number `json:"errorLogCount"`
			} `json:"smartData"`
		} `json:"drives"`
	}{}
	if err = json.Unmarshal(output, data); err != nil {
		return nil, err
	}
	for _, d := range data.Drives {
		if !d.IsMicron {
			continue
		}
		tags := opentsdb.TagSet{"serial": d.SerialNumber}
		sd := d.SmartData
		if v, err := sd.CurrentTemperature.Int64(); err == nil {
			Add(&md, "micron.current_temp", v, tags, metadata.Gauge, metadata.C, "Current drive temperature")
		}
		if v, err := sd.AvailableSpareSpace.Int64(); err == nil {
			Add(&md, "micron.available_space", v, tags, metadata.Gauge, metadata.Pct, "Percent space available")
		}
		if v, err := sd.PercentLifeUsed.Int64(); err == nil {
			Add(&md, "micron.percent_life_used", v, tags, metadata.Gauge, metadata.Pct, "Percent Life used as reported by drive")
		}
		if v, err := sd.DataUnitsRead.Int64(); err == nil {
			Add(&md, "micron.data_units_read", v, tags, metadata.Counter, metadata.None, "Data units read as reported by drive")
		}
		if v, err := sd.DataUnitsWritten.Int64(); err == nil {
			Add(&md, "micron.data_units_written", v, tags, metadata.Counter, metadata.None, "Data units written as reported by drive")
		}
		if v, err := sd.PowerCycleCount.Int64(); err == nil {
			Add(&md, "micron.power_cycle_count", v, tags, metadata.Counter, metadata.None, "Number of drive power cycles")
		}
		if v, err := sd.PowerOnHours.Int64(); err == nil {
			Add(&md, "micron.power_on_hours", v, tags, metadata.Gauge, metadata.None, "Total hours the drive has been powered on")
		}
		if v, err := sd.UnsafeShutdowns.Int64(); err == nil {
			Add(&md, "micron.unsafe_shutdowns", v, tags, metadata.Counter, metadata.None, "Number of unsafe shutdowns reported by drive")
		}
		if v, err := sd.MediaErrors.Int64(); err == nil {
			Add(&md, "micron.media_errors", v, tags, metadata.Counter, metadata.None, "Media Error count")
		}
		if v, err := sd.ErrorLogCount.Int64(); err == nil {
			Add(&md, "micron.error_log_count", v, tags, metadata.Counter, metadata.None, "Error Log Count")
		}
	}
	return md, nil
}
