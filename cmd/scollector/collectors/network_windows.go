package collectors

import (
	"regexp"

	"github.com/StackExchange/scollector/metadata"
	"github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/wmi"
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
		Add(&md, "win.net.bytes", v.BytesReceivedPerSec, opentsdb.TagSet{"iface": v.Name, "direction": "in"}, metadata.Unknown, metadata.None, "")
		Add(&md, "win.net.bytes", v.BytesSentPerSec, opentsdb.TagSet{"iface": v.Name, "direction": "out"}, metadata.Unknown, metadata.None, "")
		Add(&md, "win.net.packets", v.PacketsReceivedPerSec, opentsdb.TagSet{"iface": v.Name, "direction": "in"}, metadata.Unknown, metadata.None, "")
		Add(&md, "win.net.packets", v.PacketsSentPerSec, opentsdb.TagSet{"iface": v.Name, "direction": "out"}, metadata.Unknown, metadata.None, "")
		Add(&md, "win.net.dropped", v.PacketsOutboundDiscarded, opentsdb.TagSet{"iface": v.Name, "type": "discard", "direction": "out"}, metadata.Unknown, metadata.None, "")
		Add(&md, "win.net.dropped", v.PacketsReceivedDiscarded, opentsdb.TagSet{"iface": v.Name, "type": "discard", "direction": "in"}, metadata.Unknown, metadata.None, "")
		Add(&md, "win.net.errs", v.PacketsOutboundErrors, opentsdb.TagSet{"iface": v.Name, "type": "error", "direction": "out"}, metadata.Unknown, metadata.None, "")
		Add(&md, "win.net.errs", v.PacketsReceivedErrors, opentsdb.TagSet{"iface": v.Name, "type": "error", "direction": "in"}, metadata.Unknown, metadata.None, "")
		Add(&md, osNetBytes, v.BytesReceivedPerSec, opentsdb.TagSet{"iface": v.Name, "direction": "in"}, metadata.Unknown, metadata.None, "")
		Add(&md, osNetBytes, v.BytesSentPerSec, opentsdb.TagSet{"iface": v.Name, "direction": "out"}, metadata.Unknown, metadata.None, "")
		Add(&md, osNetPackets, v.PacketsReceivedPerSec, opentsdb.TagSet{"iface": v.Name, "direction": "in"}, metadata.Unknown, metadata.None, "")
		Add(&md, osNetPackets, v.PacketsSentPerSec, opentsdb.TagSet{"iface": v.Name, "direction": "out"}, metadata.Unknown, metadata.None, "")
		Add(&md, osNetDropped, v.PacketsOutboundDiscarded, opentsdb.TagSet{"iface": v.Name, "type": "discard", "direction": "out"}, metadata.Unknown, metadata.None, "")
		Add(&md, osNetDropped, v.PacketsReceivedDiscarded, opentsdb.TagSet{"iface": v.Name, "type": "discard", "direction": "in"}, metadata.Unknown, metadata.None, "")
		Add(&md, osNetErrors, v.PacketsOutboundErrors, opentsdb.TagSet{"iface": v.Name, "type": "error", "direction": "out"}, metadata.Unknown, metadata.None, "")
		Add(&md, osNetErrors, v.PacketsReceivedErrors, opentsdb.TagSet{"iface": v.Name, "type": "error", "direction": "in"}, metadata.Unknown, metadata.None, "")
	}
	return md, nil
}

type Win32_PerfRawData_Tcpip_NetworkInterface struct {
	BytesReceivedPerSec      uint32
	BytesSentPerSec          uint32
	Name                     string
	PacketsOutboundDiscarded uint32
	PacketsOutboundErrors    uint32
	PacketsReceivedDiscarded uint32
	PacketsReceivedErrors    uint32
	PacketsReceivedPerSec    uint32
	PacketsSentPerSec        uint32
}
