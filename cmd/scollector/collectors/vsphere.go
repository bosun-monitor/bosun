package collectors

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"bosun.org/slog"

	"github.com/vmware/govmomi/view"

	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"bosun.org/util"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/vim25/mo"
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

func c_vsphere(user, pwd, vHost string, cpuIntegrators map[string]tsIntegrator) (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint

	ctx := context.Background()

	// Make a client
	client, err := govmomi.NewClient(ctx, &url.URL{Scheme: "https", Host: vHost, Path: "/sdk"}, true)
	if err != nil {
		return nil, err
	}

	// Login with the client
	err = client.Login(ctx, url.UserPassword(user, pwd))
	if err != nil {
		return nil, err
	}

	// Get data about Host Systems (Hypervisors)
	hostSystems, err := hostSystemData(ctx, client)
	if err != nil {
		return md, nil
	}

	// A map of Keys to the Host name, so VirtualMachine.Runtime.Host can be identified
	hostKeys := make(map[string]string)

	// Data for Hosts (Hypervisors)
	for _, host := range hostSystems {
		name := util.Clean(host.Name)
		hostKeys[host.Self.Value] = name
		tags := opentsdb.TagSet{"host": name}

		// Memory
		memTotal := host.Summary.Hardware.MemorySize
		Add(&md, osMemTotal, memTotal, tags, metadata.Gauge, metadata.Bytes, osMemTotalDesc)
		memUsed := int64(host.Summary.QuickStats.OverallMemoryUsage)
		memUsed = memUsed * 1024 * 1024 // MegaBytes to Bytes
		Add(&md, osMemUsed, memUsed, tags, metadata.Gauge, metadata.Bytes, osMemUsedDesc)
		if memTotal > 0 && memUsed > 0 {
			memFree := memTotal - memUsed
			Add(&md, osMemFree, memFree, tags, metadata.Gauge, metadata.Bytes, osMemFreeDesc)
			Add(&md, osMemPctFree, float64(memFree)/float64(memTotal)*100, tags, metadata.Gauge, metadata.Pct, osMemPctFreeDesc)
		}

		// CPU
		cpuUse := int64(host.Summary.QuickStats.OverallCpuUsage)
		Add(&md, "vsphere.cpu", cpuUse, opentsdb.TagSet{"host": name, "type": "usage"}, metadata.Gauge, metadata.MHz, "")
		cpuMhz := int64(host.Summary.Hardware.CpuMhz)
		cpuCores := int64(host.Summary.Hardware.NumCpuCores)
		if cpuMhz > 0 && cpuUse > 0 && cpuCores > 0 {
			cpuTotal := cpuMhz * cpuCores
			Add(&md, "vsphere.cpu", cpuTotal-cpuUse, opentsdb.TagSet{"host": name, "type": "idle"}, metadata.Gauge, metadata.MHz, "")
			pct := float64(cpuUse) / float64(cpuTotal) * 100
			Add(&md, "vsphere.cpu.pct", pct, tags, metadata.Gauge, metadata.Pct, "")
			if _, ok := cpuIntegrators[name]; !ok {
				cpuIntegrators[name] = getTsIntegrator()
			}
			Add(&md, osCPU, cpuIntegrators[name](time.Now().Unix(), pct), tags, metadata.Counter, metadata.Pct, "")
		}

		// Uptime
		Add(&md, osSystemUptime, host.Summary.QuickStats.Uptime, tags, metadata.Gauge, metadata.Second, osSystemUptimeDesc)

		// Hardware Information
		metadata.AddMeta("", tags, "model", host.Summary.Hardware.Model, false)
		var lastServiceTag string
		for _, x := range host.Summary.Hardware.OtherIdentifyingInfo {
			if x.IdentifierType.GetElementDescription().Key == "ServiceTag" {
				lastServiceTag = x.IdentifierValue
			}
		}
		if lastServiceTag != "" {
			metadata.AddMeta("", tags, "serialNumber", lastServiceTag, false)
		}

	}

	// Get information for Virtual Machines
	vms, err := vmData(ctx, client)
	if err != nil {
		return md, nil
	}

	// Data for Virtual Machines
	for _, vm := range vms {
		name := util.Clean(vm.Name)
		if name == "" {
			slog.Errorf("Encounter virtual machine '%v' with empty name after cleaning, skipping", vm.Name)
		}
		tags := opentsdb.TagSet{"host": vHost, "guest": name}

		// Identify VM Host (Hypervisor)
		if v, ok := hostKeys[vm.Runtime.Host.Value]; ok {
			metadata.AddMeta("", opentsdb.TagSet{"host": name}, "hypervisor", v, false)
		}

		// Memory
		memTotal := int64(vm.Summary.Config.MemorySizeMB) * 1024 * 1024
		Add(&md, "vsphere.guest.mem.total", memTotal, tags, metadata.Gauge, metadata.Bytes, "")
		Add(&md, "vsphere.guest.mem.host", int64(vm.Summary.QuickStats.HostMemoryUsage)*1024*1024, tags, metadata.Gauge, metadata.Bytes, descVsphereGuestMemHost)
		memUsed := int64(vm.Summary.QuickStats.HostMemoryUsage) * 1024 * 1024
		Add(&md, "vsphere.guest.mem.used", memUsed, tags, metadata.Gauge, metadata.Bytes, descVsphereGuestMemUsed)
		Add(&md, "vsphere.guest.mem.ballooned", int64(vm.Summary.QuickStats.BalloonedMemory)*1024*1024, tags, metadata.Gauge, metadata.Bytes, descVsphereGuestMemBallooned)
		if memTotal > 0 && memUsed > 0 {
			memFree := memTotal - memUsed
			Add(&md, "vsphere.guest.mem.free", memFree, tags, metadata.Gauge, metadata.Bytes, "")
			Add(&md, "vsphere.guest.mem.percent_free", float64(memFree)/float64(memTotal)*100, tags, metadata.Gauge, metadata.Pct, "")
		}

		// CPU
		Add(&md, "vsphere.guest.cpu", vm.Summary.QuickStats.OverallCpuUsage, tags, metadata.Gauge, metadata.MHz, "")

		// Power State
		var pState int
		var missing bool
		switch vm.Runtime.PowerState {
		case "poweredOn":
			pState = 0
		case "poweredOff":
			pState = 1
		case "suspended":
			pState = 2
		default:
			missing = true
			slog.Errorf("did not recognize %s as a valid value for vsphere.guest.powered_state", vm.Runtime.PowerState)
		}
		if !missing {
			Add(&md, "vsphere.guest.powered_state", pState, tags, metadata.Gauge, metadata.StatusCode, descVsphereGuestPoweredState)
		}

		// Connection State
		missing = false
		var cState int
		switch vm.Runtime.ConnectionState {
		case "connected":
			cState = 0
		case "disconnected":
			cState = 1
		case "inaccessible":
			cState = 2
		case "invalid":
			cState = 3
		case "orphaned":
			cState = 4
		default:
			missing = true
			slog.Errorf("did not recognize %s as a valid value for vsphere.guest.connection_state", vm.Runtime.ConnectionState)
		}
		if !missing {
			Add(&md, "vsphere.guest.connection_state", cState, tags, metadata.Gauge, metadata.StatusCode, descVsphereGuestConnectionState)
		}
	}
	// Get information for Data Stores

	// host to mounted data stores
	hostStores := make(map[string][]string)

	dataStores, err := vmDataStoreData(ctx, client)
	if err != nil {
		return md, nil
	}

	for _, ds := range dataStores {
		name := util.Clean(ds.Name)
		if name == "" {
			slog.Errorf("skipping vpshere datastore %s because cleaned name was empty", ds.Name)
			continue
		}

		tags := opentsdb.TagSet{
			"disk": name,
			"host": "",
		}

		// Diskspace
		diskTotal := ds.Summary.Capacity
		Add(&md, osDiskTotal, diskTotal, tags, metadata.Gauge, metadata.Bytes, "")
		Add(&md, "vsphere.disk.space_total", diskTotal, tags, metadata.Gauge, metadata.Bytes, "")
		diskFree := ds.Summary.FreeSpace
		Add(&md, "vsphere.disk.space_free", diskFree, tags, metadata.Gauge, metadata.Bytes, "")
		if diskTotal > 0 && diskFree > 0 {
			diskUsed := diskTotal - diskFree
			Add(&md, "vsphere.disk.space_used", diskUsed, tags, metadata.Gauge, metadata.Bytes, "")
			Add(&md, osDiskUsed, diskUsed, tags, metadata.Gauge, metadata.Bytes, "")
			Add(&md, osDiskPctFree, float64(diskFree)/float64(diskTotal)*100, tags, metadata.Gauge, metadata.Pct, "")
		}

		for _, hostMount := range ds.Host {
			if host, ok := hostKeys[hostMount.Key.Value]; ok {
				if *hostMount.MountInfo.Mounted && *hostMount.MountInfo.Accessible {
					hostStores[host] = append(hostStores[host], name)
				}
			}
		}

		for host, stores := range hostStores {
			j, err := json.Marshal(stores)
			if err != nil {
				slog.Errorf("error marshaling datastores for host %v: %v", host, err)
			}
			metadata.AddMeta("", opentsdb.TagSet{"host": host}, "dataStores", string(j), false)
		}

	}

	return md, nil

}

// hostSystemData uses the client to get the 'name' and 'summary' sections of the HostSystem Type
func hostSystemData(ctx context.Context, client *govmomi.Client) ([]mo.HostSystem, error) {
	m := view.NewManager(client.Client)
	hostSystems := []mo.HostSystem{}
	view, err := m.CreateContainerView(ctx, client.ServiceContent.RootFolder, []string{"HostSystem"}, true)
	if err != nil {
		return hostSystems, err
	}

	defer view.Destroy(ctx)

	err = view.Retrieve(ctx, []string{"HostSystem"}, []string{"name", "summary"}, &hostSystems)
	if err != nil {
		return hostSystems, err
	}
	return hostSystems, nil
}

// vmData uses the client to get the 'name', 'summary', and 'runtime' sections of the VirtualMachine Type
func vmData(ctx context.Context, client *govmomi.Client) ([]mo.VirtualMachine, error) {
	m := view.NewManager(client.Client)
	vms := []mo.VirtualMachine{}
	view, err := m.CreateContainerView(ctx, client.ServiceContent.RootFolder, []string{"VirtualMachine"}, true)
	if err != nil {
		return vms, err
	}

	defer view.Destroy(ctx)

	err = view.Retrieve(ctx, []string{"VirtualMachine"}, []string{"name", "summary", "runtime"}, &vms)
	if err != nil {
		return vms, err
	}
	return vms, nil
}

// vmDataStoreData uses the client to get the 'name', 'summary', and 'runtime' sections of the Datastore Type
func vmDataStoreData(ctx context.Context, client *govmomi.Client) ([]mo.Datastore, error) {
	m := view.NewManager(client.Client)
	ds := []mo.Datastore{}
	view, err := m.CreateContainerView(ctx, client.ServiceContent.RootFolder, []string{"Datastore"}, true)
	if err != nil {
		return ds, err
	}

	defer view.Destroy(ctx)

	err = view.Retrieve(ctx, []string{"Datastore"}, []string{"name", "host", "summary"}, &ds)
	if err != nil {
		return ds, err
	}
	return ds, nil
}

const (
	descVsphereGuestMemHost         = "Host memory utilization, also known as consumed host memory. Includes the overhead memory of the VM."
	descVsphereGuestMemUsed         = "Guest memory utilization statistics, also known as active guest memory."
	descVsphereGuestMemBallooned    = "The size of the balloon driver in the VM. The host will inflate the balloon driver to reclaim physical memory from the VM. This is a sign that there is memory pressure on the host."
	descVsphereGuestPoweredState    = "PowerState defines a simple set of states for a virtual machine: poweredOn (0), poweredOff (1), and suspended (2). If the virtual machine is in a state with a task in progress, this transitions to a new state when the task completes."
	descVsphereGuestConnectionState = "The connectivity state of the virtual machine: Connected (0) means the server has access to the virtual machine, Disconnected (1) means the server is currently disconnected from the virtual machine, Inaccessible (2) means one or more of the virtual machine configuration files are inaccessible, Invalid (3) means the virtual machine configuration format is invalid, and Orphanded (4) means the virtual machine is no longer registered on the host it is associated with."
)
