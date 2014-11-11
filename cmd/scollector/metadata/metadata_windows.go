package metadata

import (
	"github.com/StackExchange/slog"
	"github.com/StackExchange/wmi"
	"github.com/bosun-monitor/scollector/opentsdb"
)

func init() {
	metafuncs = append(metafuncs, metaWindowsVersion, metaWindowsIfaces)
}

func metaWindowsVersion() {
	var dst []Win32_OperatingSystem
	q := wmi.CreateQuery(&dst, "")
	err := wmi.Query(q, &dst)
	if err != nil {
		slog.Error(err)
		return
	}
	for _, v := range dst {
		AddMeta("", nil, "version", v.Version, true)
		AddMeta("", nil, "versionCaption", v.Caption, true)
	}
}

type Win32_OperatingSystem struct {
	Caption string
	Version string
}

func metaWindowsIfaces() {
	var dstConfigs []Win32_NetworkAdapterConfiguration
	q := wmi.CreateQuery(&dstConfigs, "")
	err := wmi.Query(q, &dstConfigs)
	if err != nil {
		slog.Error(err)
		return
	}

	mNicConfigs := make(map[string]*Win32_NetworkAdapterConfiguration)
	for i, nic := range dstConfigs {
		mNicConfigs[nic.SettingID] = &dstConfigs[i]
	}

	var dstAdapters []MSFT_NetAdapter
	q = wmi.CreateQuery(&dstAdapters, "WHERE HardwareInterface = True") //Exclude virtual adapters
	err = wmi.QueryNamespace(q, &dstAdapters, "root\\StandardCimv2")
	if err != nil {
		slog.Error(err)
		return
	}

	for _, v := range dstAdapters {
		tag := opentsdb.TagSet{"iface": v.InterfaceName}
		AddMeta("", tag, "description", v.InterfaceDescription, true)
		AddMeta("", tag, "name", v.Name, true)
		AddMeta("", tag, "speed", v.Speed, true)

		nicConfig := mNicConfigs[v.InterfaceGuid]
		if nicConfig != nil {
			AddMeta("", tag, "mac", v.InterfaceGuid, true) // should be nicConfig.MACAddress
			//for _, ip := range nic.IPAddress {
			AddMeta("", tag, "addr", nicConfig.SettingID, true) // should be ip
			//}
		}
	}
}

type MSFT_NetAdapter struct {
	Name                 string //NY-WEB09-PRI-NIC-A
	Speed                uint64 //Bits per Second
	InterfaceDescription string //Intel(R) Gigabit ET Quad Port Server Adapter #2
	InterfaceName        string //Ethernet_10
	InterfaceGuid        string //unique id
}

type Win32_NetworkAdapterConfiguration struct {
	//IPAddress  []string //Both IPv4 and IPv6
	//MACAddress string //00:1B:21:93:00:00
	SettingID string //Matches InterfaceGuid
	Caption   string
}
