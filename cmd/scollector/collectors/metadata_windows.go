package collectors

import (
	"encoding/json"
	"fmt"
	"net"
	"sort"
	"strings"
	"time"

	"bosun.org/_third_party/github.com/StackExchange/wmi"
	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"bosun.org/slog"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_meta_windows_version, Interval: time.Minute * 30})
	collectors = append(collectors, &IntervalCollector{F: c_meta_windows_ifaces, Interval: time.Minute * 30})
}

func c_meta_windows_version() (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	var dst []Win32_OperatingSystem
	q := wmi.CreateQuery(&dst, "")
	err := wmi.Query(q, &dst)
	if err != nil {
		slog.Error(err)
		return md, err
	}

	var dstComputer []Win32_ComputerSystem
	q = wmi.CreateQuery(&dstComputer, "")
	err = wmi.Query(q, &dstComputer)
	if err != nil {
		slog.Error(err)
		return md, err
	}

	var dstBIOS []Win32_BIOS
	q = wmi.CreateQuery(&dstBIOS, "")
	err = wmi.Query(q, &dstBIOS)
	if err != nil {
		slog.Error(err)
		return md, err
	}

	for _, v := range dst {
		metadata.AddMeta("", nil, "version", v.Version, true)
		metadata.AddMeta("", nil, "versionCaption", v.Caption, true)
	}

	for _, v := range dstComputer {
		metadata.AddMeta("", nil, "manufacturer", v.Manufacturer, true)
		metadata.AddMeta("", nil, "model", v.Model, true)
		metadata.AddMeta("", nil, "memoryTotal", v.TotalPhysicalMemory, true)
	}

	for _, v := range dstBIOS {
		metadata.AddMeta("", nil, "serialNumber", v.SerialNumber, true)
	}
	return md, nil
}

type Win32_OperatingSystem struct {
	FreePhysicalMemory     uint64
	FreeVirtualMemory      uint64
	TotalVirtualMemorySize uint64
	TotalVisibleMemorySize uint64
	Caption                string
	Version                string
}

type Win32_ComputerSystem struct {
	Manufacturer              string
	Model                     string
	TotalPhysicalMemory       uint64
	NumberOfLogicalProcessors uint32
}

type Win32_BIOS struct {
	SerialNumber string
}

func c_meta_windows_ifaces() (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	var dstConfigs []Win32_NetworkAdapterConfiguration
	q := wmi.CreateQuery(&dstConfigs, "WHERE MACAddress != null")
	err := wmi.Query(q, &dstConfigs)
	if err != nil {
		slog.Error(err)
		return md, err
	}

	mNicConfigs := make(map[uint32]*Win32_NetworkAdapterConfiguration)
	for i, nic := range dstConfigs {
		mNicConfigs[nic.InterfaceIndex] = &dstConfigs[i]
	}

	mNicTeamIDtoSpeed := make(map[string]uint64)
	mNicTeamIDtoMaster := make(map[string]string)
	var dstTeamMembers []MSFT_NetLbfoTeamMember
	q = wmi.CreateQuery(&dstTeamMembers, "")
	err = wmi.QueryNamespace(q, &dstTeamMembers, "root\\StandardCimv2")
	if err == nil {
		for _, teamMember := range dstTeamMembers {
			mNicTeamIDtoSpeed[teamMember.InstanceID] = teamMember.ReceiveLinkSpeed
			mNicTeamIDtoMaster[teamMember.InstanceID] = teamMember.Team
		}
	}

	var dstAdapters []Win32_NetworkAdapter
	q = wmi.CreateQuery(&dstAdapters, "WHERE PhysicalAdapter=True and MACAddress <> null and NetConnectionStatus = 2") //Only adapters with MAC addresses and status="Connected"
	err = wmi.Query(q, &dstAdapters)
	if err != nil {
		slog.Error(err)
		return md, err
	}

	mNicIndextoIPs := make(map[int]string)
	ifaces, _ := net.Interfaces()
	for _, iface := range ifaces {
		if iface.Flags&(net.FlagLoopback|net.FlagPointToPoint) != 0 {
			continue
		}
		rawAds, _ := iface.Addrs()
		addrs := make([]string, len(rawAds))
		for i, rAd := range rawAds {
			addrs[i] = rAd.String()
		}
		sort.Strings(addrs)
		j, _ := json.Marshal(addrs)
		mNicIndextoIPs[iface.Index] = string(j)
	}

	for _, v := range dstAdapters {
		tag := opentsdb.TagSet{"iface": fmt.Sprint("Interface", v.InterfaceIndex)}
		metadata.AddMeta("", tag, "description", v.Description, true)
		metadata.AddMeta("", tag, "name", v.NetConnectionID, true)
		metadata.AddMeta("", tag, "mac", strings.Replace(v.MACAddress, ":", "", -1), true)
		if v.Speed != nil && *v.Speed != 0 {
			metadata.AddMeta("", tag, "speed", v.Speed, true)
		} else {
			nicSpeed := mNicTeamIDtoSpeed[v.GUID]
			metadata.AddMeta("", tag, "speed", nicSpeed, true)
		}

		nicMaster := mNicTeamIDtoMaster[v.GUID]
		if nicMaster != "" {
			metadata.AddMeta("", tag, "master", nicMaster, true)
		}

		nicIPs := mNicIndextoIPs[int(v.InterfaceIndex)]
		if nicIPs == "" {
			nicIPs = "[]"
		}
		metadata.AddMeta("", tag, "addresses", nicIPs, true)
	}
	return md, nil
}

type Win32_NetworkAdapterConfiguration struct {
	IPAddress      *[]string //Both IPv4 and IPv6
	InterfaceIndex uint32
}
type MSFT_NetLbfoTeamMember struct {
	Name             string
	ReceiveLinkSpeed uint64
	Team             string
	InstanceID       string
}
