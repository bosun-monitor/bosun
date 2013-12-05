package collectors

import (
	"github.com/StackExchange/tcollector/opentsdb"
	"github.com/StackExchange/wmi"
)

func init() {
	collectors = append(collectors, c_cpu_windows)
	collectors = append(collectors, c_network_windows)
}

const CPU_QUERY = `
	SELECT Name, PercentPrivilegedTime, PercentInterruptTime, PercentUserTime
	FROM Win32_PerfRawData_PerfOS_Processor
	WHERE Name <> '_Total'
`

const NETWORK_QUERY = `
	SELECT Name, BytesReceivedPerSec, BytesSentPerSec,
		PacketsReceivedPerSec, PacketsSentPerSec,
		PacketsOutboundDiscarded, PacketsOutboundErrors,
		PacketsReceivedDiscarded, PacketsReceivedErrors
	FROM Win32_PerfRawData_Tcpip_NetworkInterface
	WHERE NOT (Name LIKE '%local%')
`

func c_cpu_windows() opentsdb.MultiDataPoint {
	var dst []wmi.Win32_PerfRawData_PerfOS_Processor
	err := wmi.Query(CPU_QUERY, &dst)
	if err != nil {
		l.Println("cpu:", err)
		return nil
	}
	var md opentsdb.MultiDataPoint
	for _, v := range dst {
		Add(&md, "cpu.time", v.PercentPrivilegedTime, opentsdb.TagSet{"cpu": v.Name, "type": "privileged"})
		Add(&md, "cpu.time", v.PercentInterruptTime, opentsdb.TagSet{"cpu": v.Name, "type": "interrupt"})
		Add(&md, "cpu.time", v.PercentUserTime, opentsdb.TagSet{"cpu": v.Name, "type": "user"})
	}
	return md
}

func c_network_windows() opentsdb.MultiDataPoint {
	var dst []wmi.Win32_PerfRawData_Tcpip_NetworkInterface
	err := wmi.Query(NETWORK_QUERY, &dst)
	if err != nil {
		l.Println("network:", err)
		return nil
	}
	var md opentsdb.MultiDataPoint
	for _, v := range dst {
		Add(&md, "network.bytes", v.BytesReceivedPerSec, opentsdb.TagSet{"iface": v.Name, "direction": "in"})
		Add(&md, "network.bytes", v.BytesSentPerSec, opentsdb.TagSet{"iface": v.Name, "direction": "out"})
		Add(&md, "network.packets", v.PacketsReceivedPerSec, opentsdb.TagSet{"iface": v.Name, "direction": "in"})
		Add(&md, "network.packets", v.PacketsSentPerSec, opentsdb.TagSet{"iface": v.Name, "direction": "out"})
		Add(&md, "network.err", v.PacketsOutboundDiscarded, opentsdb.TagSet{"iface": v.Name, "type": "discard", "direction": "out"})
		Add(&md, "network.err", v.PacketsReceivedDiscarded, opentsdb.TagSet{"iface": v.Name, "type": "discard", "direction": "in"})
		Add(&md, "network.err", v.PacketsOutboundErrors, opentsdb.TagSet{"iface": v.Name, "type": "error", "direction": "out"})
		Add(&md, "network.err", v.PacketsReceivedErrors, opentsdb.TagSet{"iface": v.Name, "type": "error", "direction": "in"})
	}
	return md
}

