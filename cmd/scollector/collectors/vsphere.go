package collectors

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"time"

	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"bosun.org/slog"
	"bosun.org/util"
	"bosun.org/vsphere"
)

// Vsphere registers a vSphere collector.
func Vsphere(user, pwd, host string) error {
	if host == "" || user == "" || pwd == "" {
		return fmt.Errorf("empty Host, User, or Password in Vsphere")
	}
	cpuIntegrators := make(map[string]tsIntegrator)
	collectors = append(collectors, &IntervalCollector{
		F: func() (opentsdb.MultiDataPoint, error) {
			return c_vsphere(user, pwd, host, cpuIntegrators)
		},
		name: fmt.Sprintf("vsphere-%s", host),
	})
	return nil
}

func c_vsphere(user, pwd, host string, cpuIntegrators map[string]tsIntegrator) (opentsdb.MultiDataPoint, error) {
	v, err := vsphere.Connect(host, user, pwd)
	if err != nil {
		return nil, err
	}
	var md opentsdb.MultiDataPoint
	// reference ID to cleaned name
	hostKey := make(map[string]string)
	if err := vsphereHost(v, &md, cpuIntegrators, hostKey); err != nil {
		return nil, err
	}
	if err := vsphereDatastore(v, &md, hostKey); err != nil {
		return nil, err
	}
	if err := vsphereGuest(util.Clean(host), v, &md); err != nil {
		return nil, err
	}
	return md, nil
}

type DatastoreHostMount struct {
	Key       string `xml:"key"`
	MountInfo struct {
		Accessible bool   `xml:"accessible"`
		AccessMode string `xml:"accessMode"`
		Mounted    bool   `xml:"mounted"`
		Path       string `xml:"path"`
	} `xml:"mountInfo"`
}

func vsphereDatastore(v *vsphere.Vsphere, md *opentsdb.MultiDataPoint, hostKey map[string]string) error {
	res, err := v.Info("Datastore", []string{
		"name",
		"host",
		"summary.capacity",
		"summary.freeSpace",
	})
	if err != nil {
		return err
	}
	// host to mounted data stores
	hostStores := make(map[string][]string)
	var Error error
	for _, r := range res {
		var name string
		for _, p := range r.Props {
			if p.Name == "name" {
				name = p.Val.Inner
				break
			}
		}
		if name == "" {
			Error = fmt.Errorf("vsphere: empty name")
			continue
		}
		tags := opentsdb.TagSet{
			"disk": name,
			"host": "",
		}
		var diskTotal, diskFree int64
		for _, p := range r.Props {
			switch p.Val.Type {
			case "xsd:long", "xsd:int", "xsd:short":
				i, err := strconv.ParseInt(p.Val.Inner, 10, 64)
				if err != nil {
					Error = fmt.Errorf("vsphere bad integer: %s", p.Val.Inner)
					continue
				}
				switch p.Name {
				case "summary.capacity":
					Add(md, osDiskTotal, i, tags, metadata.Gauge, metadata.Bytes, "")
					Add(md, "vsphere.disk.space_total", i, tags, metadata.Gauge, metadata.Bytes, "")
					diskTotal = i
				case "summary.freeSpace":
					Add(md, "vsphere.disk.space_free", i, tags, metadata.Gauge, metadata.Bytes, "")
					diskFree = i
				}
			case "ArrayOfDatastoreHostMount":
				switch p.Name {
				case "host":
					d := xml.NewDecoder(bytes.NewBufferString(p.Val.Inner))

					for {
						var m DatastoreHostMount
						err := d.Decode(&m)
						if err == io.EOF {
							break
						}
						if err != nil {
							return err
						}
						if host, ok := hostKey[m.Key]; ok {
							if m.MountInfo.Mounted && m.MountInfo.Accessible {
								hostStores[host] = append(hostStores[host], name)
							}
						}
					}
				}
			}
		}
		if diskTotal > 0 && diskFree > 0 {
			diskUsed := diskTotal - diskFree
			Add(md, "vsphere.disk.space_used", diskUsed, tags, metadata.Gauge, metadata.Bytes, "")
			Add(md, osDiskUsed, diskUsed, tags, metadata.Gauge, metadata.Bytes, "")
			Add(md, osDiskPctFree, float64(diskFree)/float64(diskTotal)*100, tags, metadata.Gauge, metadata.Pct, "")
		}
	}
	for host, stores := range hostStores {
		j, err := json.Marshal(stores)
		if err != nil {
			slog.Errorf("error marshaling datastores for host %v: %v", host, err)
		}
		metadata.AddMeta("", opentsdb.TagSet{"host": host}, "dataStores", string(j), false)
	}
	return Error
}

type HostSystemIdentificationInfo struct {
	IdentiferValue string `xml:"identifierValue"`
	IdentiferType  struct {
		Label   string `xml:"label"`
		Summary string `xml:"summary"`
		Key     string `xml:"key"`
	} `xml:"identifierType"`
}

func vsphereHost(v *vsphere.Vsphere, md *opentsdb.MultiDataPoint, cpuIntegrators map[string]tsIntegrator, hostKey map[string]string) error {
	res, err := v.Info("HostSystem", []string{
		"name",
		"summary.hardware.cpuMhz",
		"summary.hardware.memorySize", // bytes
		"summary.hardware.numCpuCores",
		"summary.hardware.numCpuCores",
		"summary.quickStats.overallCpuUsage",    // MHz
		"summary.quickStats.overallMemoryUsage", // MB
		"summary.quickStats.uptime",             // seconds
		"summary.hardware.otherIdentifyingInfo",
		"summary.hardware.model",
	})
	if err != nil {
		return err
	}
	var Error error
	counterInfo := make(map[int]MetricInfo)
	for _, r := range res {
		var name string
		for _, p := range r.Props {
			if p.Name == "name" {
				name = util.Clean(p.Val.Inner)
				break
			}
		}
		if name == "" {
			Error = fmt.Errorf("vsphere: empty name")
			continue
		}
		hostKey[r.ID] = name
		tags := opentsdb.TagSet{
			"host": name,
		}
		var memTotal, memUsed int64
		var cpuMhz, cpuCores, cpuUse int64
		for _, p := range r.Props {
			switch p.Val.Type {
			case "xsd:long", "xsd:int", "xsd:short":
				i, err := strconv.ParseInt(p.Val.Inner, 10, 64)
				if err != nil {
					Error = fmt.Errorf("vsphere bad integer: %s", p.Val.Inner)
					continue
				}
				switch p.Name {
				case "summary.hardware.memorySize":
					Add(md, osMemTotal, i, tags, metadata.Gauge, metadata.Bytes, osMemTotalDesc)
					memTotal = i
				case "summary.quickStats.overallMemoryUsage":
					memUsed = i * 1024 * 1024
					Add(md, osMemUsed, memUsed, tags, metadata.Gauge, metadata.Bytes, osMemUsedDesc)
				case "summary.hardware.cpuMhz":
					cpuMhz = i
				case "summary.quickStats.overallCpuUsage":
					cpuUse = i
					Add(md, "vsphere.cpu", cpuUse, opentsdb.TagSet{"host": name, "type": "usage"}, metadata.Gauge, metadata.MHz, "")
				case "summary.hardware.numCpuCores":
					cpuCores = i
				case "summary.quickStats.uptime":
					Add(md, osSystemUptime, i, opentsdb.TagSet{"host": name}, metadata.Gauge, metadata.Second, osSystemUptimeDesc)
				}
			case "xsd:string":
				switch p.Name {
				case "summary.hardware.model":
					metadata.AddMeta("", tags, "model", p.Val.Inner, false)
				}
			case "ArrayOfHostSystemIdentificationInfo":
				switch p.Name {
				case "summary.hardware.otherIdentifyingInfo":
					d := xml.NewDecoder(bytes.NewBufferString(p.Val.Inner))
					// Blade servers may have multiple service tags. We want to use the last one.
					var lastServiceTag string
					for {
						var t HostSystemIdentificationInfo
						err := d.Decode(&t)
						if err == io.EOF {
							break
						}
						if err != nil {
							return err
						}
						if t.IdentiferType.Key == "ServiceTag" {
							lastServiceTag = t.IdentiferValue
						}
					}
					if lastServiceTag != "" {
						metadata.AddMeta("", tags, "serialNumber", lastServiceTag, false)
					}
				}
			}
		}
		if memTotal > 0 && memUsed > 0 {
			memFree := memTotal - memUsed
			Add(md, osMemFree, memFree, tags, metadata.Gauge, metadata.Bytes, osMemFreeDesc)
			Add(md, osMemPctFree, float64(memFree)/float64(memTotal)*100, tags, metadata.Gauge, metadata.Pct, osMemPctFreeDesc)
		}
		if cpuMhz > 0 && cpuUse > 0 && cpuCores > 0 {
			cpuTotal := cpuMhz * cpuCores
			Add(md, "vsphere.cpu", cpuTotal-cpuUse, opentsdb.TagSet{"host": name, "type": "idle"}, metadata.Gauge, metadata.MHz, "")
			pct := float64(cpuUse) / float64(cpuTotal) * 100
			Add(md, "vsphere.cpu.pct", pct, tags, metadata.Gauge, metadata.Pct, "")
			if _, ok := cpuIntegrators[name]; !ok {
				cpuIntegrators[name] = getTsIntegrator()
			}
			Add(md, osCPU, cpuIntegrators[name](time.Now().Unix(), pct), tags, metadata.Counter, metadata.Pct, "")
		}
		err := vspherePerfCounters(v, md, &tags, "vsphere.perf", "HostSystem", r.ID, counterInfo)
		if err != nil {
			Error = err
		}
	}
	return Error
}

func vsphereGuest(vsphereHost string, v *vsphere.Vsphere, md *opentsdb.MultiDataPoint) error {
	hres, err := v.Info("HostSystem", []string{
		"name",
	})
	if err != nil {
		return err
	}
	//Fetch host ids so we can set the hypervisor as metadata
	hosts := make(map[string]string)
	for _, r := range hres {
		for _, p := range r.Props {
			if p.Name == "name" {
				hosts[r.ID] = util.Clean(p.Val.Inner)
				break
			}
		}
	}
	res, err := v.Info("VirtualMachine", []string{
		"name",
		"runtime.host",
		"runtime.powerState",
		"runtime.connectionState",
		"config.hardware.memoryMB",
		"config.hardware.numCPU",
		"summary.quickStats.balloonedMemory",
		"summary.quickStats.guestMemoryUsage",
		"summary.quickStats.hostMemoryUsage",
		"summary.quickStats.overallCpuUsage",
	})
	if err != nil {
		return err
	}
	var Error error
	counterInfo := make(map[int]MetricInfo)
	for _, r := range res {
		var name string
		for _, p := range r.Props {
			if p.Name == "name" {
				name = util.Clean(p.Val.Inner)
				break
			}
		}
		if name == "" {
			Error = fmt.Errorf("vsphere: empty name")
			continue
		}
		tags := opentsdb.TagSet{
			"host": vsphereHost, "guest": name,
		}
		var memTotal, memUsed int64
		for _, p := range r.Props {
			switch p.Val.Type {
			case "xsd:long", "xsd:int", "xsd:short":
				i, err := strconv.ParseInt(p.Val.Inner, 10, 64)
				if err != nil {
					Error = fmt.Errorf("vsphere bad integer: %s", p.Val.Inner)
					continue
				}
				switch p.Name {
				case "config.hardware.memoryMB":
					memTotal = i * 1024 * 1024
					Add(md, "vsphere.guest.mem.total", memTotal, tags, metadata.Gauge, metadata.Bytes, "")
				case "summary.quickStats.hostMemoryUsage":
					Add(md, "vsphere.guest.mem.host", i*1024*1024, tags, metadata.Gauge, metadata.Bytes, descVsphereGuestMemHost)
				case "summary.quickStats.guestMemoryUsage":
					memUsed = i * 1024 * 1024
					Add(md, "vsphere.guest.mem.used", memUsed, tags, metadata.Gauge, metadata.Bytes, descVsphereGuestMemUsed)
				case "summary.quickStats.overallCpuUsage":
					Add(md, "vsphere.guest.cpu", i, tags, metadata.Gauge, metadata.MHz, "")
				case "summary.quickStats.balloonedMemory":
					Add(md, "vsphere.guest.mem.ballooned", i*1024*1024, tags, metadata.Gauge, metadata.Bytes, descVsphereGuestMemBallooned)
				case "config.hardware.numCPU":
					Add(md, "vsphere.guest.num_cpu", i, tags, metadata.Gauge, metadata.Gauge, "")
				}
			case "HostSystem":
				s := p.Val.Inner
				switch p.Name {
				case "runtime.host":
					if v, ok := hosts[s]; ok {
						metadata.AddMeta("", opentsdb.TagSet{"host": name}, "hypervisor", v, false)
					}
				}
			case "VirtualMachinePowerState":
				s := p.Val.Inner
				var missing bool
				var v int
				switch s {
				case "poweredOn":
					v = 0
				case "poweredOff":
					v = 1
				case "suspended":
					v = 2
				default:
					missing = true
					slog.Errorf("Did not recognize %s as a valid value for vsphere.guest.powered_state", s)
				}
				if !missing {
					Add(md, "vsphere.guest.powered_state", v, tags, metadata.Gauge, metadata.StatusCode, descVsphereGuestPoweredState)
				}
			case "VirtualMachineConnectionState":
				s := p.Val.Inner
				var missing bool
				var v int
				switch s {
				case "connected":
					v = 0
				case "disconnected":
					v = 1
				case "inaccessible":
					v = 2
				case "invalid":
					v = 3
				case "orphaned":
					v = 4
				default:
					missing = true
					slog.Errorf("Did not recognize %s as a valid value for vsphere.guest.connection_state", s)
				}
				if !missing {
					Add(md, "vsphere.guest.connection_state", v, tags, metadata.Gauge, metadata.StatusCode, descVsphereGuestConnectionState)
				}
			}
		}
		if memTotal > 0 && memUsed > 0 {
			memFree := memTotal - memUsed
			Add(md, "vsphere.guest.mem.free", memFree, tags, metadata.Gauge, metadata.Bytes, "")
			Add(md, "vsphere.guest.mem.percent_free", float64(memFree)/float64(memTotal)*100, tags, metadata.Gauge, metadata.Pct, "")
		}
		err := vspherePerfCounters(v, md, &tags, "vsphere.guest.perf", "VirtualMachine", r.ID, counterInfo)
		if err != nil {
			Error = err
		}
	}
	return Error
}

type MetricInfo struct {
	Metric      string
	Unit        string
	Rate        string
	Description string
}

func vspherePerfCounters(v *vsphere.Vsphere, md *opentsdb.MultiDataPoint, tags *opentsdb.TagSet, metricprefix string, etype string, ename string, ci map[int]MetricInfo) error {
	pm, err := v.PerformanceProvider(etype, ename)
	if err != nil {
		return fmt.Errorf("vsphere: couldn't get Performance Manager for %s %s: %v", etype, ename, err)
	}

	pems, err := v.PerfCountersValues(etype, ename, pm)
	if err != nil {
		return fmt.Errorf("vsphere: couldn't get PerfCountersValues for %s %s: %v", etype, ename, err)
	}

	if pems == nil || pems.Value == nil {
		// Empty counters list
		return nil
	}

	var counters bytes.Buffer
	for _, pem := range pems.Value {
		if _, ok := ci[pem.Id.CounterId] ; !ok {
			counters.WriteString(fmt.Sprintf("<counterId>%d</counterId>", pem.Id.CounterId))
		}
	}

	if counters.Len() > 0 {
		pcis, err := v.PerfCounterInfos(counters.String())
		if err != nil {
			return fmt.Errorf("vsphere: couldn't get PerfCounterInfos for %s %s: %v", etype, ename, err)
		}
		for _, pci := range pcis {
			if _, ok := ci[pci.Key] ; !ok {
				var mi MetricInfo
				mi.Metric = fmt.Sprintf("%s.%s.%s", metricprefix, pci.GroupInfo.Key, pci.NameInfo.Key)
				mi.Unit = pci.UnitInfo.Key
				mi.Rate = pci.StatsType
				mi.Description = pci.NameInfo.Summary + fmt.Sprintf(" (%s %s, original units: %s)", pci.RollupType, mi.Rate, mi.Unit)
				ci[pci.Key] = mi
			}
		}
	}

	for _, pem := range pems.Value {
		var pemrate metadata.RateType
		var pemunit metadata.Unit
		var value float64
		ctri := ci[pem.Id.CounterId]
		value = float64(pem.Value)
		switch ctri.Rate {
		case "absolute":
			pemrate = metadata.Counter
		case "delta":
			pemrate = metadata.Gauge
		case "rate":
			pemrate = metadata.Rate
		default:
			pemrate = metadata.Gauge
		}
		switch ctri.Unit {
		case "joule":
			pemunit = metadata.None
		case "kiloBytes":
			pemunit = metadata.Bytes
			value = value * 1024
		case "kiloBytesPerSecond":
			pemunit = metadata.BytesPerSecond
			value = value * 1024
		case "megaBytes":
			pemunit = metadata.Bytes
			value = value * 1024 * 1024
		case "megaBytesPerSecond":
			pemunit = metadata.BytesPerSecond
			value = value * 1024 * 1024
		case "megaHertz":
			pemunit = metadata.MHz
		case "microsecond":
			pemunit = metadata.MilliSecond
			value = value / 1000
		case "millisecond":
			pemunit = metadata.MilliSecond
		case "number":
			pemunit = metadata.None
		case "percent":
			pemunit = metadata.Pct
		case "second":
			pemunit = metadata.Second
		case "watt":
			pemunit = metadata.Watt
		default:
			pemunit = metadata.None
		}
		tagset := tags.Copy()
		if pem.Id.Instance != "" {
			tagset = tagset.Merge(opentsdb.TagSet{"instance": pem.Id.Instance})
		}
		Add(md, ctri.Metric, value, tagset, pemrate, pemunit, ctri.Description)
	}
	return nil
}

const (
	descVsphereGuestMemHost         = "Host memory utilization, also known as consumed host memory. Includes the overhead memory of the VM."
	descVsphereGuestMemUsed         = "Guest memory utilization statistics, also known as active guest memory."
	descVsphereGuestMemBallooned    = "The size of the balloon driver in the VM. The host will inflate the balloon driver to reclaim physical memory from the VM. This is a sign that there is memory pressure on the host."
	descVsphereGuestPoweredState    = "PowerState defines a simple set of states for a virtual machine: poweredOn (0), poweredOff (1), and suspended (2). If the virtual machine is in a state with a task in progress, this transitions to a new state when the task completes."
	descVsphereGuestConnectionState = "The connectivity state of the virtual machine: Connected (0) means the server has access to the virtual machine, Disconnected (1) means the server is currently disconnected from the virtual machine, Inaccessible (2) means one or more of the virtual machine configuration files are inaccessible, Invalid (3) means the virtual machine configuration format is invalid, and Orphanded (4) means the virtual machine is no longer registered on the host it is associated with."
)
