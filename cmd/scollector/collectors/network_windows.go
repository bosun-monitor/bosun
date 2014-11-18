package collectors

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/bosun-monitor/scollector/_third_party/github.com/StackExchange/slog"
	"github.com/bosun-monitor/scollector/_third_party/github.com/StackExchange/wmi"
	"github.com/bosun-monitor/scollector/_third_party/github.com/bosun-monitor/metadata"
	"github.com/bosun-monitor/scollector/_third_party/github.com/bosun-monitor/opentsdb"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_network_windows, init: winNetworkInit})
}

var interfaceExclusions = regexp.MustCompile("isatap|Teredo")

// instanceNameToUnderscore matches # / and \ and is used to find characters that should be replaced with an underscore.
var instanceNameToUnderscore = regexp.MustCompile("[#/\\\\]")
var mNicInstanceNameToInterfaceIndex = make(map[string]string)

// winNetworkInit maintains a lookup table for mapping InstanceName to InterfaceIndex for all active network adapters.
func winNetworkInit() {
	update := func() {
		var dstNetworkAdapter []Win32_NetworkAdapter
		q := wmi.CreateQuery(&dstNetworkAdapter, "WHERE PhysicalAdapter=True and MACAddress <> null")
		err := queryWmi(q, &dstNetworkAdapter)
		if err != nil {
			slog.Error(err)
			return
		}
		for _, nic := range dstNetworkAdapter {
			var iface = fmt.Sprint("Interface", nic.InterfaceIndex)
			//Get PnPName using Win32_PnPEntity class
			var pnpname = ""
			var escapeddeviceid = strings.Replace(nic.PNPDeviceID, "\\", "\\\\", -1)
			var filter = fmt.Sprintf("WHERE DeviceID='%s'", escapeddeviceid)
			var dstPnPName []Win32_PnPEntity
			q = wmi.CreateQuery(&dstPnPName, filter)
			err = queryWmi(q, &dstPnPName)
			if err != nil {
				slog.Error(err)
				return
			}
			for _, pnp := range dstPnPName { //Really should be a single item
				pnpname = pnp.Name
			}
			if pnpname == "" {
				slog.Errorf("%s cannot find Win32_PnPEntity %s", iface, filter)
				continue
			}

			//Convert to instance name (see http://msdn.microsoft.com/en-us/library/system.diagnostics.performancecounter.instancename(v=vs.110).aspx )
			instanceName := pnpname
			instanceName = strings.Replace(instanceName, "(", "[", -1)
			instanceName = strings.Replace(instanceName, ")", "]", -1)
			instanceName = instanceNameToUnderscore.ReplaceAllString(instanceName, "_")
			mNicInstanceNameToInterfaceIndex[instanceName] = iface
		}
	}
	update()
	go func() {
		for _ = range time.Tick(time.Minute * 5) {
			update()
		}
	}()
}

func c_network_windows() (opentsdb.MultiDataPoint, error) {
	var dstStats []Win32_PerfRawData_Tcpip_NetworkInterface
	var q = wmi.CreateQuery(&dstStats, "")
	err := queryWmi(q, &dstStats)
	if err != nil {
		return nil, err
	}

	var md opentsdb.MultiDataPoint
	for _, nicStats := range dstStats {
		if interfaceExclusions.MatchString(nicStats.Name) {
			continue
		}

		iface := mNicInstanceNameToInterfaceIndex[nicStats.Name]
		if iface == "" {
			continue
		}
		//This does NOT include TEAM network adapters. Those will go to os.net.bond using new WMI classes only available in Server 2012+
		Add(&md, "win.net.bytes", nicStats.BytesReceivedPersec, opentsdb.TagSet{"iface": iface, "direction": "in"}, metadata.Counter, metadata.BytesPerSecond, descWinNetBytesReceivedPersec)
		Add(&md, "win.net.bytes", nicStats.BytesSentPersec, opentsdb.TagSet{"iface": iface, "direction": "out"}, metadata.Counter, metadata.BytesPerSecond, descWinNetBytesSentPersec)
		Add(&md, "win.net.packets", nicStats.PacketsReceivedPersec, opentsdb.TagSet{"iface": iface, "direction": "in"}, metadata.Counter, metadata.PerSecond, descWinNetPacketsReceivedPersec)
		Add(&md, "win.net.packets", nicStats.PacketsSentPersec, opentsdb.TagSet{"iface": iface, "direction": "out"}, metadata.Counter, metadata.PerSecond, descWinNetPacketsSentPersec)
		Add(&md, "win.net.dropped", nicStats.PacketsOutboundDiscarded, opentsdb.TagSet{"iface": iface, "type": "discard", "direction": "out"}, metadata.Counter, metadata.PerSecond, descWinNetPacketsOutboundDiscarded)
		Add(&md, "win.net.dropped", nicStats.PacketsReceivedDiscarded, opentsdb.TagSet{"iface": iface, "type": "discard", "direction": "in"}, metadata.Counter, metadata.PerSecond, descWinNetPacketsReceivedDiscarded)
		Add(&md, "win.net.errs", nicStats.PacketsOutboundErrors, opentsdb.TagSet{"iface": iface, "type": "error", "direction": "out"}, metadata.Counter, metadata.PerSecond, descWinNetPacketsOutboundErrors)
		Add(&md, "win.net.errs", nicStats.PacketsReceivedErrors, opentsdb.TagSet{"iface": iface, "type": "error", "direction": "in"}, metadata.Counter, metadata.PerSecond, descWinNetPacketsReceivedErrors)
		Add(&md, osNetBytes, nicStats.BytesReceivedPersec, opentsdb.TagSet{"iface": iface, "direction": "in"}, metadata.Counter, metadata.BytesPerSecond, osNetBytesDesc)
		Add(&md, osNetBytes, nicStats.BytesSentPersec, opentsdb.TagSet{"iface": iface, "direction": "out"}, metadata.Counter, metadata.BytesPerSecond, osNetBytesDesc)
		Add(&md, osNetPackets, nicStats.PacketsReceivedPersec, opentsdb.TagSet{"iface": iface, "direction": "in"}, metadata.Counter, metadata.PerSecond, osNetPacketsDesc)
		Add(&md, osNetPackets, nicStats.PacketsSentPersec, opentsdb.TagSet{"iface": iface, "direction": "out"}, metadata.Counter, metadata.PerSecond, osNetPacketsDesc)
		Add(&md, osNetDropped, nicStats.PacketsOutboundDiscarded, opentsdb.TagSet{"iface": iface, "type": "discard", "direction": "out"}, metadata.Counter, metadata.PerSecond, osNetDroppedDesc)
		Add(&md, osNetDropped, nicStats.PacketsReceivedDiscarded, opentsdb.TagSet{"iface": iface, "type": "discard", "direction": "in"}, metadata.Counter, metadata.PerSecond, osNetDroppedDesc)
		Add(&md, osNetErrors, nicStats.PacketsOutboundErrors, opentsdb.TagSet{"iface": iface, "type": "error", "direction": "out"}, metadata.Counter, metadata.PerSecond, osNetErrorsDesc)
		Add(&md, osNetErrors, nicStats.PacketsReceivedErrors, opentsdb.TagSet{"iface": iface, "type": "error", "direction": "in"}, metadata.Counter, metadata.PerSecond, osNetErrorsDesc)
	}
	return md, nil
}

const (
	descWinNetBytesReceivedPersec      = "Bytes Received/sec is the rate at which bytes are received over each network adapter, including framing characters. Network Interface\\Bytes Received/sec is a subset of Network Interface\\Bytes Total/sec."
	descWinNetBytesSentPersec          = "Bytes Sent/sec is the rate at which bytes are sent over each network adapter, including framing characters. Network Interface\\Bytes Sent/sec is a subset of Network Interface\\Bytes Total/sec."
	descWinNetPacketsReceivedPersec    = "Packets Received/sec is the rate at which packets are received on the network interface."
	descWinNetPacketsSentPersec        = "Packets Sent/sec is the rate at which packets are sent on the network interface."
	descWinNetPacketsOutboundDiscarded = "Packets Outbound Discarded is the number of outbound packets that were chosen to be discarded even though no errors had been detected to prevent transmission. One possible reason for discarding packets could be to free up buffer space."
	descWinNetPacketsReceivedDiscarded = "Packets Received Discarded is the number of inbound packets that were chosen to be discarded even though no errors had been detected to prevent their delivery to a higher-layer protocol.  One possible reason for discarding packets could be to free up buffer space."
	descWinNetPacketsOutboundErrors    = "Packets Outbound Errors is the number of outbound packets that could not be transmitted because of errors."
	descWinNetPacketsReceivedErrors    = "Packets Received Errors is the number of inbound packets that contained errors preventing them from being deliverable to a higher-layer protocol."
)

type Win32_PnPEntity struct {
	Name string //Intel(R) Gigabit ET Quad Port Server Adapter #3
}

type Win32_NetworkAdapter struct {
	Description    string //Intel(R) Gigabit ET Quad Port Server Adapter (no index)
	InterfaceIndex uint32
	PNPDeviceID    string
}

type Win32_PerfRawData_Tcpip_NetworkInterface struct {
	BytesReceivedPersec      uint32
	BytesSentPersec          uint32
	Name                     string
	PacketsOutboundDiscarded uint32
	PacketsOutboundErrors    uint32
	PacketsReceivedDiscarded uint32
	PacketsReceivedErrors    uint32
	PacketsReceivedPersec    uint32
	PacketsSentPersec        uint32
}
