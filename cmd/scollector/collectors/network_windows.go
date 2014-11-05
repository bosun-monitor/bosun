package collectors

import (
	"regexp"

	"github.com/StackExchange/wmi"
	"github.com/bosun-monitor/scollector/metadata"
	"github.com/bosun-monitor/scollector/opentsdb"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_network_windows})
}

var interfaceExclusions = regexp.MustCompile("isatap|Teredo")

func c_network_windows() (opentsdb.MultiDataPoint, error) {
	var dst []Win32_PerfRawData_Tcpip_NetworkInterface
	var q = wmi.CreateQuery(&dst, "")
	err := queryWmi(q, &dst)
	if err != nil {
		return nil, err
	}
	var md opentsdb.MultiDataPoint
	for _, v := range dst {
		if interfaceExclusions.MatchString(v.Name) {
			continue
		}
		//TODO: Somehow we will need filter out TEAMS so they can os.net.bond and not os.net to ensure
		//aggreagation doesn't get broken
		Add(&md, "win.net.bytes", v.BytesReceivedPersec, opentsdb.TagSet{"iface": v.Name, "direction": "in"}, metadata.Counter, metadata.BytesPerSecond, descWinNetBytesReceivedPersec)
		Add(&md, "win.net.bytes", v.BytesSentPersec, opentsdb.TagSet{"iface": v.Name, "direction": "out"}, metadata.Counter, metadata.BytesPerSecond, descWinNetBytesSentPersec)
		Add(&md, "win.net.packets", v.PacketsReceivedPersec, opentsdb.TagSet{"iface": v.Name, "direction": "in"}, metadata.Counter, metadata.PerSecond, descWinNetPacketsReceivedPersec)
		Add(&md, "win.net.packets", v.PacketsSentPersec, opentsdb.TagSet{"iface": v.Name, "direction": "out"}, metadata.Counter, metadata.PerSecond, descWinNetPacketsSentPersec)
		Add(&md, "win.net.dropped", v.PacketsOutboundDiscarded, opentsdb.TagSet{"iface": v.Name, "type": "discard", "direction": "out"}, metadata.Counter, metadata.PerSecond, descWinNetPacketsOutboundDiscarded)
		Add(&md, "win.net.dropped", v.PacketsReceivedDiscarded, opentsdb.TagSet{"iface": v.Name, "type": "discard", "direction": "in"}, metadata.Counter, metadata.PerSecond, descWinNetPacketsReceivedDiscarded)
		Add(&md, "win.net.errs", v.PacketsOutboundErrors, opentsdb.TagSet{"iface": v.Name, "type": "error", "direction": "out"}, metadata.Counter, metadata.PerSecond, descWinNetPacketsOutboundErrors)
		Add(&md, "win.net.errs", v.PacketsReceivedErrors, opentsdb.TagSet{"iface": v.Name, "type": "error", "direction": "in"}, metadata.Counter, metadata.PerSecond, descWinNetPacketsReceivedErrors)
		Add(&md, osNetBytes, v.BytesReceivedPersec, opentsdb.TagSet{"iface": v.Name, "direction": "in"}, metadata.Counter, metadata.BytesPerSecond, osNetBytesDesc)
		Add(&md, osNetBytes, v.BytesSentPersec, opentsdb.TagSet{"iface": v.Name, "direction": "out"}, metadata.Counter, metadata.BytesPerSecond, osNetBytesDesc)
		Add(&md, osNetPackets, v.PacketsReceivedPersec, opentsdb.TagSet{"iface": v.Name, "direction": "in"}, metadata.Counter, metadata.PerSecond, osNetPacketsDesc)
		Add(&md, osNetPackets, v.PacketsSentPersec, opentsdb.TagSet{"iface": v.Name, "direction": "out"}, metadata.Counter, metadata.PerSecond, osNetPacketsDesc)
		Add(&md, osNetDropped, v.PacketsOutboundDiscarded, opentsdb.TagSet{"iface": v.Name, "type": "discard", "direction": "out"}, metadata.Counter, metadata.PerSecond, osNetDroppedDesc)
		Add(&md, osNetDropped, v.PacketsReceivedDiscarded, opentsdb.TagSet{"iface": v.Name, "type": "discard", "direction": "in"}, metadata.Counter, metadata.PerSecond, osNetDroppedDesc)
		Add(&md, osNetErrors, v.PacketsOutboundErrors, opentsdb.TagSet{"iface": v.Name, "type": "error", "direction": "out"}, metadata.Counter, metadata.PerSecond, osNetErrorsDesc)
		Add(&md, osNetErrors, v.PacketsReceivedErrors, opentsdb.TagSet{"iface": v.Name, "type": "error", "direction": "in"}, metadata.Counter, metadata.PerSecond, osNetErrorsDesc)
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
