package collectors

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"

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
	collectors = append(collectors, &IntervalCollector{
		F: func() (opentsdb.MultiDataPoint, error) {
			return c_vsphere(user, pwd, host)
		},
		name: fmt.Sprintf("vsphere-%s", host),
	})
	return nil
}

func c_vsphere(user, pwd, host string) (opentsdb.MultiDataPoint, error) {
	v, err := vsphere.Connect(host, user, pwd)
	if err != nil {
		return nil, err
	}
	var md opentsdb.MultiDataPoint
	if err := vsphereHost(v, &md); err != nil {
		return nil, err
	}
	if err := vsphereDatastore(v, &md); err != nil {
		return nil, err
	}
	if err := vsphereGuest(util.Clean(host), v, &md); err != nil {
		return nil, err
	}
	return md, nil
}

func vsphereDatastore(v *vsphere.Vsphere, md *opentsdb.MultiDataPoint) error {
	res, err := v.Info("Datastore", []string{
		"name",
		"summary.capacity",
		"summary.freeSpace",
	})
	if err != nil {
		return err
	}
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
			}
		}
		if diskTotal > 0 && diskFree > 0 {
			diskUsed := diskTotal - diskFree
			Add(md, "vsphere.disk.space_used", diskUsed, tags, metadata.Gauge, metadata.Bytes, "")
			Add(md, osDiskUsed, diskUsed, tags, metadata.Gauge, metadata.Bytes, "")
			Add(md, osDiskPctFree, float64(diskFree)/float64(diskTotal)*100, tags, metadata.Gauge, metadata.Pct, "")
		}
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

func vsphereHost(v *vsphere.Vsphere, md *opentsdb.MultiDataPoint) error {
	res, err := v.Info("HostSystem", []string{
		"name",
		"summary.hardware.cpuMhz",
		"summary.hardware.memorySize", // bytes
		"summary.hardware.numCpuCores",
		"summary.hardware.numCpuCores",
		"summary.quickStats.overallCpuUsage",    // MHz
		"summary.quickStats.overallMemoryUsage", // MB
		"summary.hardware.otherIdentifyingInfo",
		"summary.hardware.model",
	})
	if err != nil {
		return err
	}
	var Error error
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
			Add(md, "vsphere.cpu.pct", float64(cpuUse)/float64(cpuTotal)*100, tags, metadata.Gauge, metadata.Pct, "")
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
				case "poweredOff":
					v = 0
				case "poweredOn":
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
	}
	return Error
}

const (
	descVsphereGuestMemHost         = "Host memory utilization, also known as consumed host memory. Includes the overhead memory of the VM."
	descVsphereGuestMemUsed         = "Guest memory utilization statistics, also known as active guest memory."
	descVsphereGuestMemBallooned    = "The size of the balloon driver in the VM. The host will inflate the balloon driver to reclaim physical memory from the VM. This is a sign that there is memory pressure on the host."
	descVsphereGuestPoweredState    = "PowerState defines a simple set of states for a virtual machine: poweredOn (0), poweredOff (1), and suspended (2). If the virtual machine is in a state with a task in progress, this transitions to a new state when the task completes."
	descVsphereGuestConnectionState = "The connectivity state of the virtual machine: Connected (0) means the server has access to the virtual machine, Disconnected (1) means the server is currently disconnected from the virtual machine, Inaccessible (2) means one or more of the virtual machine configuration files are inaccessible, Invalid (3) means the virtual machine configuration format is invalid, and Orphanded (4) means the virtual machine is no longer registered on the host it is associated with."
)
