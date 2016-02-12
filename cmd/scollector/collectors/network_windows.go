package collectors

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"

	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"bosun.org/slog"
	"github.com/StackExchange/wmi"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_network_windows, init: winNetworkInit})

	c_winnetteam := &IntervalCollector{
		F: c_network_team_windows,
	}
	// Make sure MSFT_NetImPlatAdapter and MSFT_NetAdapterStatisticsSettingData
	// are valid WMI classes when initializing c_network_team_windows
	c_winnetteam.init = func() {
		var dstTeamNic []MSFT_NetLbfoTeamNic
		var dstStats []MSFT_NetAdapterStatisticsSettingData
		queryTeamAdapter = wmi.CreateQuery(&dstTeamNic, "")
		queryTeamStats = wmi.CreateQuery(&dstStats, "")
		c_winnetteam.Enable = func() bool {
			errTeamNic := queryWmiNamespace(queryTeamAdapter, &dstTeamNic, namespaceStandardCimv2)
			errStats := queryWmiNamespace(queryTeamStats, &dstStats, namespaceStandardCimv2)
			result := errTeamNic == nil && errStats == nil
			return result
		}
	}
	collectors = append(collectors, c_winnetteam)
	c := &IntervalCollector{
		F: c_network_windows_tcp,
	}
	c.init = wmiInit(c, func() interface{} { return &[]Win32_PerfRawData_Tcpip_TCPv4{} }, "", &winNetTCPQuery)
	collectors = append(collectors, c)
}

var (
	queryTeamStats         string
	queryTeamAdapter       string
	winNetTCPQuery         string
	namespaceStandardCimv2 = "root\\StandardCimv2"
	interfaceExclusions    = regexp.MustCompile("isatap|Teredo")

	// instanceNameToUnderscore matches '#' '/' and '\' for replacing with '_'.
	instanceNameToUnderscore         = regexp.MustCompile("[#/\\\\]")
	mNicInstanceNameToInterfaceIndex = make(map[string]string)
)

// winNetworkToInstanceName converts a Network Adapter Name to the InstanceName
// that is used in Win32_PerfRawData_Tcpip_NetworkInterface.
func winNetworkToInstanceName(Name string) string {
	instanceName := Name
	instanceName = strings.Replace(instanceName, "(", "[", -1)
	instanceName = strings.Replace(instanceName, ")", "]", -1)
	instanceName = instanceNameToUnderscore.ReplaceAllString(instanceName, "_")
	return instanceName
}

// winNetworkInit maintains a mapping of InstanceName to InterfaceIndex
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
			// Get PnPName using Win32_PnPEntity class
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
			for _, pnp := range dstPnPName { // Really should be a single item
				pnpname = pnp.Name
			}
			if pnpname == "" {
				slog.Errorf("%s cannot find Win32_PnPEntity %s", iface, filter)
				continue
			}

			// Convert to instance name (see http://goo.gl/jfq6pq )
			instanceName := winNetworkToInstanceName(pnpname)
			mNicInstanceNameToInterfaceIndex[instanceName] = iface
		}
	}
	update()
	go func() {
		for range time.Tick(time.Minute * 5) {
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
		// This does NOT include TEAM network adapters. Those will go to os.net.bond
		tagsIn := opentsdb.TagSet{"iface": iface, "direction": "in"}
		tagsOut := opentsdb.TagSet{"iface": iface, "direction": "out"}
		Add(&md, "win.net.ifspeed", nicStats.CurrentBandwidth, opentsdb.TagSet{"iface": iface}, metadata.Gauge, metadata.BitsPerSecond, descWinNetCurrentBandwidth)
		Add(&md, "win.net.bytes", nicStats.BytesReceivedPersec, tagsIn, metadata.Counter, metadata.BytesPerSecond, descWinNetBytesReceivedPersec)
		Add(&md, "win.net.bytes", nicStats.BytesSentPersec, tagsOut, metadata.Counter, metadata.BytesPerSecond, descWinNetBytesSentPersec)
		Add(&md, "win.net.packets", nicStats.PacketsReceivedPersec, tagsIn, metadata.Counter, metadata.PerSecond, descWinNetPacketsReceivedPersec)
		Add(&md, "win.net.packets", nicStats.PacketsSentPersec, tagsOut, metadata.Counter, metadata.PerSecond, descWinNetPacketsSentPersec)
		Add(&md, "win.net.dropped", nicStats.PacketsOutboundDiscarded, tagsOut, metadata.Counter, metadata.PerSecond, descWinNetPacketsOutboundDiscarded)
		Add(&md, "win.net.dropped", nicStats.PacketsReceivedDiscarded, tagsIn, metadata.Counter, metadata.PerSecond, descWinNetPacketsReceivedDiscarded)
		Add(&md, "win.net.errs", nicStats.PacketsOutboundErrors, tagsOut, metadata.Counter, metadata.PerSecond, descWinNetPacketsOutboundErrors)
		Add(&md, "win.net.errs", nicStats.PacketsReceivedErrors, tagsIn, metadata.Counter, metadata.PerSecond, descWinNetPacketsReceivedErrors)
		Add(&md, osNetIfSpeed, nicStats.CurrentBandwidth/1000000, opentsdb.TagSet{"iface": iface}, metadata.Gauge, metadata.Megabit, osNetIfSpeedDesc)
		Add(&md, osNetBytes, nicStats.BytesReceivedPersec, tagsIn, metadata.Counter, metadata.BytesPerSecond, osNetBytesDesc)
		Add(&md, osNetBytes, nicStats.BytesSentPersec, tagsOut, metadata.Counter, metadata.BytesPerSecond, osNetBytesDesc)
		Add(&md, osNetPackets, nicStats.PacketsReceivedPersec, tagsIn, metadata.Counter, metadata.PerSecond, osNetPacketsDesc)
		Add(&md, osNetPackets, nicStats.PacketsSentPersec, tagsOut, metadata.Counter, metadata.PerSecond, osNetPacketsDesc)
		Add(&md, osNetDropped, nicStats.PacketsOutboundDiscarded, tagsOut, metadata.Counter, metadata.PerSecond, osNetDroppedDesc)
		Add(&md, osNetDropped, nicStats.PacketsReceivedDiscarded, tagsIn, metadata.Counter, metadata.PerSecond, osNetDroppedDesc)
		Add(&md, osNetErrors, nicStats.PacketsOutboundErrors, tagsOut, metadata.Counter, metadata.PerSecond, osNetErrorsDesc)
		Add(&md, osNetErrors, nicStats.PacketsReceivedErrors, tagsIn, metadata.Counter, metadata.PerSecond, osNetErrorsDesc)
	}
	return md, nil
}

const (
	descWinNetCurrentBandwidth         = "Estimate of the interface's current bandwidth in bits per second (bps). For interfaces that do not vary in bandwidth or for those where no accurate estimation can be made, this value is the nominal bandwidth."
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
	Name string // Intel(R) Gigabit ET Quad Port Server Adapter #3
}

type Win32_NetworkAdapter struct {
	Description     string // Intel(R) Gigabit ET Quad Port Server Adapter (no index)
	InterfaceIndex  uint32
	PNPDeviceID     string
	NetConnectionID string  //NY-WEB09-PRI-NIC-A
	Speed           *uint64 //Bits per Second
	MACAddress      string  //00:1B:21:93:00:00
	GUID            string
}

type Win32_PerfRawData_Tcpip_NetworkInterface struct {
	CurrentBandwidth         uint64
	BytesReceivedPersec      uint64
	BytesSentPersec          uint64
	Name                     string
	PacketsOutboundDiscarded uint64
	PacketsOutboundErrors    uint64
	PacketsReceivedDiscarded uint64
	PacketsReceivedErrors    uint64
	PacketsReceivedPersec    uint64
	PacketsSentPersec        uint64
}

// c_network_team_windows will add metrics for team network adapters from
// MSFT_NetAdapterStatisticsSettingData for any adapters that are in
// MSFT_NetLbfoTeamNic and have a valid instanceName.
func c_network_team_windows() (opentsdb.MultiDataPoint, error) {
	var dstTeamNic []*MSFT_NetLbfoTeamNic
	err := queryWmiNamespace(queryTeamAdapter, &dstTeamNic, namespaceStandardCimv2)
	if err != nil {
		return nil, err
	}

	var dstStats []MSFT_NetAdapterStatisticsSettingData
	err = queryWmiNamespace(queryTeamStats, &dstStats, namespaceStandardCimv2)
	if err != nil {
		return nil, err
	}

	mDescriptionToTeamNic := make(map[string]*MSFT_NetLbfoTeamNic)
	for _, teamNic := range dstTeamNic {
		mDescriptionToTeamNic[teamNic.InterfaceDescription] = teamNic
	}

	var md opentsdb.MultiDataPoint
	for _, nicStats := range dstStats {
		TeamNic := mDescriptionToTeamNic[nicStats.InterfaceDescription]
		if TeamNic == nil {
			continue
		}

		instanceName := winNetworkToInstanceName(nicStats.InterfaceDescription)
		iface := mNicInstanceNameToInterfaceIndex[instanceName]
		if iface == "" {
			continue
		}
		tagsIn := opentsdb.TagSet{"iface": iface, "direction": "in"}
		tagsOut := opentsdb.TagSet{"iface": iface, "direction": "out"}
		linkSpeed := math.Min(float64(TeamNic.ReceiveLinkSpeed), float64(TeamNic.Transmitlinkspeed))
		Add(&md, "win.net.bond.ifspeed", linkSpeed, opentsdb.TagSet{"iface": iface}, metadata.Gauge, metadata.BitsPerSecond, descWinNetTeamlinkspeed)
		Add(&md, "win.net.bond.bytes", nicStats.ReceivedBytes, tagsIn, metadata.Counter, metadata.BytesPerSecond, descWinNetTeamReceivedBytes)
		Add(&md, "win.net.bond.bytes", nicStats.SentBytes, tagsOut, metadata.Counter, metadata.BytesPerSecond, descWinNetTeamSentBytes)
		Add(&md, "win.net.bond.bytes_unicast", nicStats.ReceivedUnicastBytes, tagsIn, metadata.Counter, metadata.BytesPerSecond, descWinNetTeamReceivedUnicastBytes)
		Add(&md, "win.net.bond.bytes_unicast", nicStats.SentUnicastBytes, tagsOut, metadata.Counter, metadata.BytesPerSecond, descWinNetTeamSentUnicastBytes)
		Add(&md, "win.net.bond.bytes_broadcast", nicStats.ReceivedBroadcastBytes, tagsIn, metadata.Counter, metadata.BytesPerSecond, descWinNetTeamReceivedBroadcastBytes)
		Add(&md, "win.net.bond.bytes_broadcast", nicStats.SentBroadcastBytes, tagsOut, metadata.Counter, metadata.BytesPerSecond, descWinNetTeamSentBroadcastBytes)
		Add(&md, "win.net.bond.bytes_multicast", nicStats.ReceivedMulticastBytes, tagsIn, metadata.Counter, metadata.BytesPerSecond, descWinNetTeamReceivedMulticastBytes)
		Add(&md, "win.net.bond.bytes_multicast", nicStats.SentMulticastBytes, tagsOut, metadata.Counter, metadata.BytesPerSecond, descWinNetTeamSentMulticastBytes)
		Add(&md, "win.net.bond.packets_unicast", nicStats.ReceivedUnicastPackets, tagsIn, metadata.Counter, metadata.PerSecond, descWinNetTeamReceivedUnicastPackets)
		Add(&md, "win.net.bond.packets_unicast", nicStats.SentUnicastPackets, tagsOut, metadata.Counter, metadata.PerSecond, descWinNetTeamSentUnicastPackets)
		Add(&md, "win.net.bond.dropped", nicStats.ReceivedDiscardedPackets, tagsIn, metadata.Counter, metadata.PerSecond, descWinNetTeamReceivedDiscardedPackets)
		Add(&md, "win.net.bond.dropped", nicStats.OutboundDiscardedPackets, tagsOut, metadata.Counter, metadata.PerSecond, descWinNetTeamOutboundDiscardedPackets)
		Add(&md, "win.net.bond.errs", nicStats.ReceivedPacketErrors, tagsIn, metadata.Counter, metadata.PerSecond, descWinNetTeamReceivedPacketErrors)
		Add(&md, "win.net.bond.errs", nicStats.OutboundPacketErrors, tagsOut, metadata.Counter, metadata.PerSecond, descWinNetTeamOutboundPacketErrors)
		Add(&md, "win.net.bond.packets_multicast", nicStats.ReceivedMulticastPackets, tagsIn, metadata.Counter, metadata.PerSecond, descWinNetTeamReceivedMulticastPackets)
		Add(&md, "win.net.bond.packets_multicast", nicStats.SentMulticastPackets, tagsOut, metadata.Counter, metadata.PerSecond, descWinNetTeamSentMulticastPackets)
		Add(&md, "win.net.bond.packets_broadcast", nicStats.ReceivedBroadcastPackets, tagsIn, metadata.Counter, metadata.PerSecond, descWinNetTeamReceivedBroadcastPackets)
		Add(&md, "win.net.bond.packets_broadcast", nicStats.SentBroadcastPackets, tagsOut, metadata.Counter, metadata.PerSecond, descWinNetTeamSentBroadcastPackets)
		Add(&md, osNetBondIfSpeed, linkSpeed/1000000, opentsdb.TagSet{"iface": iface}, metadata.Gauge, metadata.Megabit, osNetIfSpeedDesc)
		Add(&md, osNetBondBytes, nicStats.ReceivedBytes, tagsIn, metadata.Counter, metadata.Bytes, osNetBytesDesc)
		Add(&md, osNetBondBytes, nicStats.SentBytes, tagsOut, metadata.Counter, metadata.Bytes, osNetBytesDesc)
		Add(&md, osNetBondUnicast, nicStats.ReceivedUnicastPackets, tagsIn, metadata.Counter, metadata.Count, osNetUnicastDesc)
		Add(&md, osNetBondUnicast, nicStats.SentUnicastPackets, tagsOut, metadata.Counter, metadata.Count, osNetUnicastDesc)
		Add(&md, osNetBondMulticast, nicStats.ReceivedMulticastPackets, tagsIn, metadata.Counter, metadata.Count, osNetMulticastDesc)
		Add(&md, osNetBondMulticast, nicStats.SentMulticastPackets, tagsOut, metadata.Counter, metadata.Count, osNetMulticastDesc)
		Add(&md, osNetBondBroadcast, nicStats.ReceivedBroadcastPackets, tagsIn, metadata.Counter, metadata.Count, osNetBroadcastDesc)
		Add(&md, osNetBondBroadcast, nicStats.SentBroadcastPackets, tagsOut, metadata.Counter, metadata.Count, osNetBroadcastDesc)
		Add(&md, osNetBondPackets, float64(nicStats.ReceivedUnicastPackets)+float64(nicStats.ReceivedMulticastPackets)+float64(nicStats.ReceivedBroadcastPackets), tagsIn, metadata.Counter, metadata.Count, osNetPacketsDesc)
		Add(&md, osNetBondPackets, float64(nicStats.SentUnicastPackets)+float64(nicStats.SentMulticastPackets)+float64(nicStats.SentBroadcastPackets), tagsOut, metadata.Counter, metadata.Count, osNetPacketsDesc)
		Add(&md, osNetBondDropped, nicStats.ReceivedDiscardedPackets, tagsIn, metadata.Counter, metadata.Count, osNetDroppedDesc)
		Add(&md, osNetBondDropped, nicStats.OutboundDiscardedPackets, tagsOut, metadata.Counter, metadata.Count, osNetDroppedDesc)
		Add(&md, osNetBondErrors, nicStats.ReceivedPacketErrors, tagsIn, metadata.Counter, metadata.Count, osNetErrorsDesc)
		Add(&md, osNetBondErrors, nicStats.OutboundPacketErrors, tagsOut, metadata.Counter, metadata.Count, osNetErrorsDesc)
	}
	return md, nil
}

const (
	descWinNetTeamlinkspeed                = "The link speed of the adapter in bits per second."
	descWinNetTeamReceivedBytes            = "The number of bytes of data received without errors through this interface. This value includes bytes in unicast, broadcast, and multicast packets."
	descWinNetTeamReceivedUnicastPackets   = "The number of unicast packets received without errors through this interface."
	descWinNetTeamReceivedMulticastPackets = "The number of multicast packets received without errors through this interface."
	descWinNetTeamReceivedBroadcastPackets = "The number of broadcast packets received without errors through this interface."
	descWinNetTeamReceivedUnicastBytes     = "The number of unicast bytes received without errors through this interface."
	descWinNetTeamReceivedMulticastBytes   = "The number of multicast bytes received without errors through this interface."
	descWinNetTeamReceivedBroadcastBytes   = "The number of broadcast bytes received without errors through this interface."
	descWinNetTeamReceivedDiscardedPackets = "The number of inbound packets which were chosen to be discarded even though no errors were detected to prevent the packets from being deliverable to a higher-layer protocol."
	descWinNetTeamReceivedPacketErrors     = "The number of incoming packets that were discarded because of errors."
	descWinNetTeamSentBytes                = "The number of bytes of data transmitted without errors through this interface. This value includes bytes in unicast, broadcast, and multicast packets."
	descWinNetTeamSentUnicastPackets       = "The number of unicast packets transmitted without errors through this interface."
	descWinNetTeamSentMulticastPackets     = "The number of multicast packets transmitted without errors through this interface."
	descWinNetTeamSentBroadcastPackets     = "The number of broadcast packets transmitted without errors through this interface."
	descWinNetTeamSentUnicastBytes         = "The number of unicast bytes transmitted without errors through this interface."
	descWinNetTeamSentMulticastBytes       = "The number of multicast bytes transmitted without errors through this interface."
	descWinNetTeamSentBroadcastBytes       = "The number of broadcast bytes transmitted without errors through this interface."
	descWinNetTeamOutboundDiscardedPackets = "The number of outgoing packets that were discarded even though they did not have errors."
	descWinNetTeamOutboundPacketErrors     = "The number of outgoing packets that were discarded because of errors."
)

type MSFT_NetLbfoTeamNic struct {
	Team                 string
	Name                 string
	ReceiveLinkSpeed     uint64
	Transmitlinkspeed    uint64
	InterfaceDescription string
}

type MSFT_NetAdapterStatisticsSettingData struct {
	InstanceID               string
	Name                     string
	InterfaceDescription     string
	ReceivedBytes            uint64
	ReceivedUnicastPackets   uint64
	ReceivedMulticastPackets uint64
	ReceivedBroadcastPackets uint64
	ReceivedUnicastBytes     uint64
	ReceivedMulticastBytes   uint64
	ReceivedBroadcastBytes   uint64
	ReceivedDiscardedPackets uint64
	ReceivedPacketErrors     uint64
	SentBytes                uint64
	SentUnicastPackets       uint64
	SentMulticastPackets     uint64
	SentBroadcastPackets     uint64
	SentUnicastBytes         uint64
	SentMulticastBytes       uint64
	SentBroadcastBytes       uint64
	OutboundDiscardedPackets uint64
	OutboundPacketErrors     uint64
}

var (
	winNetTCPSegmentsLastCount           uint32
	winNetTCPSegmentsLastRetransmitCount uint32
)

func c_network_windows_tcp() (opentsdb.MultiDataPoint, error) {
	var dst []Win32_PerfRawData_Tcpip_TCPv4
	err := queryWmi(winNetTCPQuery, &dst)
	if err != nil {
		return nil, err
	}
	var md opentsdb.MultiDataPoint
	for _, v := range dst {
		Add(&md, "win.net.tcp.failures", v.ConnectionFailures, nil, metadata.Counter, metadata.Connection, descWinNetTCPv4ConnectionFailures)
		Add(&md, "win.net.tcp.active", v.ConnectionsActive, nil, metadata.Counter, metadata.Connection, descWinNetTCPv4ConnectionsActive)
		Add(&md, "win.net.tcp.established", v.ConnectionsEstablished, nil, metadata.Gauge, metadata.Connection, descWinNetTCPv4ConnectionsEstablished)
		Add(&md, "win.net.tcp.passive", v.ConnectionsPassive, nil, metadata.Counter, metadata.Connection, descWinNetTCPv4ConnectionsPassive)
		Add(&md, "win.net.tcp.reset", v.ConnectionsReset, nil, metadata.Gauge, metadata.Connection, descWinNetTCPv4ConnectionsReset)
		Add(&md, "win.net.tcp.segments", v.SegmentsReceivedPersec, opentsdb.TagSet{"type": "received"}, metadata.Counter, metadata.PerSecond, descWinNetTCPv4SegmentsReceivedPersec)
		Add(&md, "win.net.tcp.segments", v.SegmentsRetransmittedPersec, opentsdb.TagSet{"type": "retransmitted"}, metadata.Counter, metadata.PerSecond, descWinNetTCPv4SegmentsRetransmittedPersec)
		Add(&md, "win.net.tcp.segments", v.SegmentsSentPersec, opentsdb.TagSet{"type": "sent"}, metadata.Counter, metadata.PerSecond, descWinNetTCPv4SegmentsSentPersec)
		if winNetTCPSegmentsLastCount != 0 &&
			(v.SegmentsPersec-winNetTCPSegmentsLastCount) != 0 &&
			(v.SegmentsRetransmittedPersec > winNetTCPSegmentsLastRetransmitCount) &&
			(v.SegmentsPersec > winNetTCPSegmentsLastCount) {
			val := float64(v.SegmentsRetransmittedPersec-winNetTCPSegmentsLastRetransmitCount) / float64(v.SegmentsPersec-winNetTCPSegmentsLastCount) * 100
			Add(&md, "win.net.tcp.retransmit_pct", val, nil, metadata.Gauge, metadata.Pct, descWinNetTCPv4SegmentsRetransmit)
		}
		winNetTCPSegmentsLastRetransmitCount = v.SegmentsRetransmittedPersec
		winNetTCPSegmentsLastCount = v.SegmentsPersec
	}
	return md, nil
}

const (
	descWinNetTCPv4ConnectionFailures          = "Connection Failures is the number of times TCP connections have made a direct transition to the CLOSED state from the SYN-SENT state or the SYN-RCVD state, plus the number of times TCP connections have made a direct transition to the LISTEN state from the SYN-RCVD state."
	descWinNetTCPv4ConnectionsActive           = "Connections Active is the number of times TCP connections have made a direct transition to the SYN-SENT state from the CLOSED state. In other words, it shows a number of connections which are initiated by the local computer. The value is a cumulative total."
	descWinNetTCPv4ConnectionsEstablished      = "Connections Established is the number of TCP connections for which the current state is either ESTABLISHED or CLOSE-WAIT."
	descWinNetTCPv4ConnectionsPassive          = "Connections Passive is the number of times TCP connections have made a direct transition to the SYN-RCVD state from the LISTEN state. In other words, it shows a number of connections to the local computer, which are initiated by remote computers. The value is a cumulative total."
	descWinNetTCPv4ConnectionsReset            = "Connections Reset is the number of times TCP connections have made a direct transition to the CLOSED state from either the ESTABLISHED state or the CLOSE-WAIT state."
	descWinNetTCPv4SegmentsReceivedPersec      = "Segments Received/sec is the rate at which segments are received, including those received in error.  This count includes segments received on currently established connections."
	descWinNetTCPv4SegmentsRetransmittedPersec = "Segments Retransmitted/sec is the rate at which segments are retransmitted, that is, segments transmitted containing one or more previously transmitted bytes."
	descWinNetTCPv4SegmentsSentPersec          = "Segments Sent/sec is the rate at which segments are sent, including those on current connections, but excluding those containing only retransmitted bytes."
	descWinNetTCPv4SegmentsRetransmit          = "Segments Retransmitted / (Segments Sent + Segments Received). Usually expected to be less than 0.1 - 0.01 percent, and anything above 1 percent is an indicator of a poor connection."
)

type Win32_PerfRawData_Tcpip_TCPv4 struct {
	ConnectionFailures          uint32
	ConnectionsActive           uint32
	ConnectionsEstablished      uint32
	ConnectionsPassive          uint32
	ConnectionsReset            uint32
	SegmentsPersec              uint32
	SegmentsReceivedPersec      uint32
	SegmentsRetransmittedPersec uint32
	SegmentsSentPersec          uint32
}
