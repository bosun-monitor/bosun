package collectors

import (
	"fmt"
	"io"
	"strings"
	"time"

	"bosun.org/cmd/scollector/conf"
	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"bosun.org/snmp"
)

// SNMPCiscoBGP adds the collector for BGP health data from Cisco devices
func SNMPCiscoBGP(cfg conf.SNMP) {
	collectors = append(collectors, &IntervalCollector{
		F: func() (opentsdb.MultiDataPoint, error) {
			return c_snmp_ciscobgp(cfg.Community, cfg.Host)
		},
		Interval: time.Second * 30,
		name:     fmt.Sprintf("snmp-ciscobgp-%s", cfg.Host),
	})
}

func c_snmp_ciscobgp(community, host string) (opentsdb.MultiDataPoint, error) {
	const (
		state               = ".1.3.6.1.4.1.9.9.187.1.2.5.1.3.1.4"
		adminStatus         = ".1.3.6.1.4.1.9.9.187.1.2.5.1.4.1.4"
		localAS             = ".1.3.6.1.4.1.9.9.187.1.2.5.1.8.1.4"
		localIdentifier     = ".1.3.6.1.4.1.9.9.187.1.2.5.1.9.1.4"
		remoteAS            = ".1.3.6.1.4.1.9.9.187.1.2.5.1.11.1.4"
		remoteIdentifier    = ".1.3.6.1.4.1.9.9.187.1.2.5.1.12.1.4"
		inUpdates           = ".1.3.6.1.4.1.9.9.187.1.2.5.1.13.1.4"
		outUpdates          = ".1.3.6.1.4.1.9.9.187.1.2.5.1.14.1.4"
		inTotalMessages     = ".1.3.6.1.4.1.9.9.187.1.2.5.1.15.1.4"
		outTotalMessages    = ".1.3.6.1.4.1.9.9.187.1.2.5.1.16.1.4"
		inUpdateElapsedTime = ".1.3.6.1.4.1.9.9.187.1.2.5.1.27.1.4"
		acceptedPrefixes    = ".1.3.6.1.4.1.9.9.187.1.2.8.1.1.1.4"
		deniedPrefixes      = ".1.3.6.1.4.1.9.9.187.1.2.8.1.2.1.4"
		advertisedPrefixes  = ".1.3.6.1.4.1.9.9.187.1.2.8.1.6.1.4"
		suppressedPrefixes  = ".1.3.6.1.4.1.9.9.187.1.2.8.1.7.1.4"
		withdrawnPrefixes   = ".1.3.6.1.4.1.9.9.187.1.2.8.1.8.1.4"

		bgpPeerState               = "bgp.peer.state"
		bgpPeerAdminStatus         = "bgp.peer.admin_status"
		bgpPeerInUpdates           = "bgp.peer.updates.in"
		bgpPeerOutUpdates          = "bgp.peer.updates.out"
		bgpPeerInTotalMessages     = "bgp.peer.messages.in"
		bgpPeerOutTotalMessages    = "bgp.peer.messages.out"
		bgpPeerInUpdateElapsedTime = "bgp.peer.update_elapsed.in"
		bgpPeerAcceptedPrefixes    = "bgp.peer.prefixes.accepted"
		bgpPeerDeniedPrefixes      = "bgp.peer.prefixes.denied"
		bgpPeerAdvertisedPrefixes  = "bgp.peer.prefixes.advertised"
		bgpPeerSuppressedPrefixes  = "bgp.peer.prefixes.suppressed"
		bgpPeerWithdrawnPrefixes   = "bgp.peer.prefixes.withdrawn"

		bgpPeerStateDesc               = "The state of the peer. 1:idle 2:connect 3:active 4:opensent 5:openconfirm 6:established"
		bgpPeerAdminStatusDesc         = "The admin status of the peer connection 1:stop 2:start"
		bgpPeerInUpdatesDesc           = "The number of updates received from the peer in the current session"
		bgpPeerOutUpdatesDesc          = "The number of updates sent to the peer in the current session"
		bgpPeerInTotalMessagesDesc     = "The number of total messages received from the peer in the current session"
		bgpPeerOutTotalMessagesDesc    = "The number of total messages sent to the peer in the current session"
		bgpPeerInUpdateElapsedTimeDesc = "The amount of time since the last update from the peer"
		bgpPeerAcceptedPrefixesDesc    = "The number of prefixes accepted from the peer"
		bgpPeerDeniedPrefixesDesc      = "The number of prefixes from the peer that were denied this session"
		bgpPeerAdvertisedPrefixesDesc  = "The number of prefixes that have been advertised to the peer this session"
		bgpPeerSuppressedPrefixesDesc  = "The number of prefixes that have been suppressed from sending to the peer this session"
		bgpPeerWithdrawnPrefixesDesc   = "The number of prefixes that the local node has withdrawn from the peer this session"
	)
	// Tag: local_as
	localASesRaw, err := snmp_ip_tree(host, community, localAS)
	if err != nil {
		return nil, err
	}
	localASes := make(map[string]string, len(localASesRaw))
	for k, v := range localASesRaw {
		s := fmt.Sprint(v)
		localASes[k] = s
	}

	// Tag: local_id
	localIdentifiersRaw, err := snmp_ip_tree(host, community, localIdentifier)
	if err != nil {
		return nil, err
	}
	localIdentifiers := make(map[string]string, len(localIdentifiersRaw))
	for k, v := range localIdentifiersRaw {
		if uv, ok := v.([]uint8); ok {
			localIdentifiers[k] = snmp_combine_ip_uint8(uv)
		} else {
			return nil, fmt.Errorf("Bad IP address data in local identifier for peer %q on host %q", k, host)
		}
	}

	// Tag: remote_as
	remoteASesRaw, err := snmp_ip_tree(host, community, remoteAS)
	if err != nil {
		return nil, err
	}
	remoteASes := make(map[string]string, len(remoteASesRaw))
	for k, v := range remoteASesRaw {
		s := fmt.Sprint(v)
		remoteASes[k] = s
	}

	// Tag: remote_id
	remoteIdentifiersRaw, err := snmp_ip_tree(host, community, remoteIdentifier)
	if err != nil {
		return nil, err
	}
	remoteIdentifiers := make(map[string]string, len(remoteIdentifiersRaw))
	for k, v := range remoteIdentifiersRaw {
		if uv, ok := v.([]uint8); ok {
			remoteIdentifiers[k] = snmp_combine_ip_uint8(uv)
		} else {
			return nil, fmt.Errorf("Bad IP address data in remote identifier for peer %q on host %q", k, host)
		}
	}

	var md opentsdb.MultiDataPoint

	type bgpAdd struct {
		oid    string
		metric string
		rate   metadata.RateType
		unit   metadata.Unit
		desc   string
	}

	// Function to harvest all metrics with the tag groups above
	add := func(bA bgpAdd) error {
		m, err := snmp_ip_tree(host, community, bA.oid)
		if err != nil {
			return err
		}
		for k, v := range m {
			_, localASok := localASes[k]
			_, localIdentifierok := localIdentifiers[k]
			_, remoteASok := remoteASes[k]
			_, remoteIdentifierok := remoteIdentifiers[k]
			if localASok && localIdentifierok && remoteASok && remoteIdentifierok {
				tags := opentsdb.TagSet{
					"host":      host,
					"peer":      k,
					"local_as":  localASes[k],
					"local_id":  localIdentifiers[k],
					"remote_as": remoteASes[k],
					"remote_id": remoteIdentifiers[k],
				}
				Add(&md, bA.metric, v, tags, bA.rate, bA.unit, bA.desc)
			} else {
				return fmt.Errorf("Missing tag data for peer %q on host %q", k, host)
			}
		}
		return nil
	}
	// oid, metric, rate, unit, description
	oids := []bgpAdd{
		{state, bgpPeerState, metadata.Gauge, metadata.StatusCode, bgpPeerStateDesc},
		{adminStatus, bgpPeerAdminStatus, metadata.Gauge, metadata.StatusCode, bgpPeerAdminStatusDesc},
		{inUpdates, bgpPeerInUpdates, metadata.Counter, metadata.Packet, bgpPeerInUpdatesDesc},
		{outUpdates, bgpPeerOutUpdates, metadata.Counter, metadata.Packet, bgpPeerOutUpdatesDesc},
		{inTotalMessages, bgpPeerInTotalMessages, metadata.Counter, metadata.Packet, bgpPeerInTotalMessagesDesc},
		{outTotalMessages, bgpPeerOutTotalMessages, metadata.Counter, metadata.Packet, bgpPeerOutTotalMessagesDesc},
		{inUpdateElapsedTime, bgpPeerInUpdateElapsedTime, metadata.Gauge, metadata.Second, bgpPeerInUpdateElapsedTimeDesc},
		{acceptedPrefixes, bgpPeerAcceptedPrefixes, metadata.Gauge, metadata.Count, bgpPeerAcceptedPrefixesDesc},
		{deniedPrefixes, bgpPeerDeniedPrefixes, metadata.Counter, metadata.Count, bgpPeerDeniedPrefixesDesc},
		{advertisedPrefixes, bgpPeerAdvertisedPrefixes, metadata.Counter, metadata.Count, bgpPeerAdvertisedPrefixesDesc},
		{suppressedPrefixes, bgpPeerSuppressedPrefixes, metadata.Counter, metadata.Count, bgpPeerSuppressedPrefixesDesc},
		{withdrawnPrefixes, bgpPeerWithdrawnPrefixes, metadata.Counter, metadata.Count, bgpPeerWithdrawnPrefixesDesc},
	}
	for _, bA := range oids {
		if err := add(bA); err != nil {
			return nil, err
		}
	}
	return md, nil
}

func snmp_ip_tree(host, community, oid string) (map[string]interface{}, error) {
	rows, err := snmp.Walk(host, community, oid)
	if err != nil {
		return nil, err
	}
	m := make(map[string]interface{})
	for rows.Next() {
		key := ""
		var a interface{}
		id, err := rows.Scan(&a)
		if err != nil {
			return nil, err
		}
		switch t := id.(type) {
		case []int:
			key = snmp_combine_ip_int(t)
		default:
			return nil, fmt.Errorf("Got wrong type from OID check")
		}
		m[key] = a
	}
	if err := rows.Err(); err != nil && err != io.EOF {
		return nil, err
	}
	return m, nil
}

func snmp_combine_ip_uint8(path []uint8) string {
	s := make([]string, len(path))
	for i := range path {
		s[i] = fmt.Sprint(path[i])
	}
	// In some cases there's a .1.1 after the IP (because SNMP is awesome), drop it:
	s = s[:4]
	return strings.Join(s, ".")
}

func snmp_combine_ip_int(path []int) string {
	s := make([]string, len(path))
	for i := range path {
		s[i] = fmt.Sprint(path[i])
	}
	// In some cases there's a .1.1 after the IP (because SNMP is awesome), drop it:
	s = s[:4]
	return strings.Join(s, ".")
}
