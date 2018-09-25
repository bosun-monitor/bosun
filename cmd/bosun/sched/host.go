package sched // import "bosun.org/cmd/bosun/sched"
import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"bosun.org/slog"
)

func (s *Schedule) Host(filter string) (map[string]*HostData, error) {
	timeFilterAge := time.Hour * 2 * 24
	hosts := make(map[string]*HostData)
	allHosts, err := s.Search.TagValuesByTagKey("host", timeFilterAge)
	if err != nil {
		return nil, err
	}
	for _, h := range allHosts {
		hosts[h] = newHostData()
	}
	states, err := s.GetOpenStates()
	if err != nil {
		return nil, err
	}
	silences := s.Silenced()
	// These are all fetched by metric since that is how we store it in redis,
	// so this makes for the fastest response
	tagsByKey := func(metric, hostKey string) (map[string][]opentsdb.TagSet, error) {
		byKey := make(map[string][]opentsdb.TagSet)
		tags, err := s.Search.FilteredTagSets(metric, nil, 0)
		if err != nil {
			return byKey, err
		}
		for _, ts := range tags {
			if host, ok := ts[hostKey]; ok {
				// Make sure the host exists based on our time filter
				if _, ok := hosts[host]; ok {
					byKey[host] = append(byKey[host], ts)
				}
			}
		}
		return byKey, nil
	}
	oldTimestamp := utcNow().Add(-timeFilterAge).Unix()
	oldOrErr := func(ts int64, err error) bool {
		if ts < oldTimestamp || err != nil {
			return true
		}
		return false
	}
	osNetBytesTags, err := tagsByKey("os.net.bytes", "host")
	if err != nil {
		return nil, err
	}
	osNetVirtualBytesTags, err := tagsByKey("os.net.virtual.bytes", "host")
	if err != nil {
		return nil, err
	}
	osNetBondBytesTags, err := tagsByKey("os.net.bond.bytes", "host")
	if err != nil {
		return nil, err
	}
	osNetTunnelBytesTags, err := tagsByKey("os.net.tunnel.bytes", "host")
	if err != nil {
		return nil, err
	}
	osNetOtherBytesTags, err := tagsByKey("os.net.other.bytes", "host")
	if err != nil {
		return nil, err
	}
	osNetIfSpeedTags, err := tagsByKey("os.net.ifspeed", "host")
	if err != nil {
		return nil, err
	}
	osNetVirtualIfSpeedTags, err := tagsByKey("os.net.virtual.ifspeed", "host")
	if err != nil {
		return nil, err
	}
	osNetBondIfSpeedTags, err := tagsByKey("os.net.bond.ifspeed", "host")
	if err != nil {
		return nil, err
	}
	osNetTunnelIfSpeedTags, err := tagsByKey("os.net.tunnel.ifspeed", "host")
	if err != nil {
		return nil, err
	}
	osNetOtherIfSpeedTags, err := tagsByKey("os.net.other.ifspeed", "host")
	if err != nil {
		return nil, err
	}
	hwChassisTags, err := tagsByKey("hw.chassis", "host")
	if err != nil {
		return nil, err
	}
	hwPhysicalDiskTags, err := tagsByKey("hw.storage.pdisk", "host")
	if err != nil {
		return nil, err
	}
	hwVirtualDiskTags, err := tagsByKey("hw.storage.vdisk", "host")
	if err != nil {
		return nil, err
	}
	hwControllersTags, err := tagsByKey("hw.storage.controller", "host")
	if err != nil {
		return nil, err
	}
	hwBatteriesTags, err := tagsByKey("hw.storage.battery", "host")
	if err != nil {
		return nil, err
	}
	hwPowerSuppliesTags, err := tagsByKey("hw.ps", "host")
	if err != nil {
		return nil, err
	}
	hwTempsTags, err := tagsByKey("hw.chassis.temps.reading", "host")
	if err != nil {
		return nil, err
	}
	hwBoardPowerTags, err := tagsByKey("hw.chassis.power.reading", "host")
	if err != nil {
		return nil, err
	}
	diskTags, err := tagsByKey("os.disk.fs.space_total", "host")
	if err != nil {
		return nil, err
	}
	serviceTags, err := tagsByKey("os.service.running", "host")
	if err != nil {
		return nil, err
	}
	// Will assume the same tagsets exist .mem_real, .mem_virtual and possibly .count
	processTags, err := tagsByKey("os.proc.cpu", "host")
	if err != nil {
		return nil, err
	}
	// Will make the assumption that the metric bosun.ping.timeout, resolved, and rtt
	// all share the same tagset
	icmpTimeOutTags, err := tagsByKey("bosun.ping.timeout", "dst_host")
	if err != nil {
		return nil, err
	}
	for name, host := range hosts {
		host.Name = name
		hostTagSet := opentsdb.TagSet{"host": host.Name}
		hostMetadata, err := s.GetMetadata("", hostTagSet)
		if err != nil {
			slog.Error(err)
		}
		processHostIncidents(host, states, silences)
		for _, ts := range icmpTimeOutTags[host.Name] {
			// The host tag represents the polling source for these set of metrics
			source, ok := ts["host"]
			if !ok {
				slog.Errorf("couldn't find source tag for icmp data for host %s", host.Name)
			}
			// 1 Means it timed out
			timeout, timestamp, err := s.Search.GetLast("bosun.ping.timeout", ts.String(), false)
			if oldOrErr(timestamp, err) {
				continue
			}
			rtt, rttTimestamp, _ := s.Search.GetLast("bosun.ping.rtt", ts.String(), false)
			// 1 means dns resolution was successful
			dnsLookup, dnsTimestamp, dnsErr := s.Search.GetLast("bosun.ping.resolved", ts.String(), false)
			host.ICMPData[source] = &ICMPData{
				TimedOut:               timeout == 1 && err == nil,
				TimedOutLastUpdated:    timestamp,
				DNSResolved:            dnsLookup == 1 && dnsErr == nil,
				DNSResolvedLastUpdated: dnsTimestamp,
				RTTMS:                  rtt,
				RTTLastUpdated:         rttTimestamp,
			}

		}
		for _, ts := range serviceTags[host.Name] {
			name, ok := ts["name"]
			if !ok {
				slog.Errorf("couldn't find service name tag %s for host %s", name, host.Name)
				continue
			}
			fstatus, timestamp, err := s.Search.GetLast("os.service.running", ts.String(), false)
			running := false
			if fstatus != 0 {
				running = true
			}
			if !oldOrErr(timestamp, err) {
				host.Services[name] = &ServiceStatus{
					Running:            running,
					RunningLastUpdated: timestamp,
				}
			}
		}
		for _, ts := range processTags[host.Name] {
			name, ok := ts["name"]
			if !ok {
				slog.Errorf("couldn't find process name tag %s for host %s", name, host.Name)
				continue
			}
			p := &Process{}
			p.CPUPercentUsed, p.CPUPercentLastUpdated, err = s.Search.GetLast("os.proc.cpu", ts.String(), true)
			if oldOrErr(p.CPUPercentLastUpdated, err) {
				continue
			}
			p.UsedRealBytes, p.UsedRealBytesLastUpdated, _ = s.Search.GetLastInt64("os.proc.mem.real", ts.String(), false)
			p.UsedVirtualBytes, p.UsedVirtualBytesLastUpdated, _ = s.Search.GetLastInt64("os.proc.mem.virtual", ts.String(), false)
			p.Count, p.CountLastUpdated, _ = s.Search.GetLastInt64("os.proc.count", ts.String(), false)
			host.Processes[name] = p
		}
		// Process Hardware Chassis States
		for _, ts := range hwChassisTags[host.Name] {
			component, ok := ts["component"]
			if !ok {
				return nil, fmt.Errorf("couldn't find component tag for host %s", host.Name)
			}
			fstatus, timestamp, err := s.Search.GetLast("hw.chassis", ts.String(), false)
			if !oldOrErr(timestamp, err) {
				host.Hardware.ChassisComponents[component] = &ChassisComponent{
					Status:            statusString(int64(fstatus), 0, "Ok", "Bad"),
					StatusLastUpdated: timestamp,
				}
			}
		}
		for _, ts := range hwTempsTags[host.Name] {
			name, ok := ts["name"]
			if !ok {
				slog.Errorf("couldn't find name tag %s for host %s", name, host.Name)
			}
			t := &Temp{}
			var tStatus float64
			tStatus, t.StatusLastUpdated, err = s.Search.GetLast("hw.chassis.temps", ts.String(), false)
			t.Celsius, t.CelsiusLastUpdated, _ = s.Search.GetLast("hw.chassis.temps.reading", ts.String(), false)
			if oldOrErr(t.StatusLastUpdated, err) {
				continue
			}
			t.Status = statusString(int64(tStatus), 0, "Ok", "Bad")
			host.Hardware.Temps[name] = t
		}
		for _, ts := range hwPowerSuppliesTags[host.Name] {
			id, ok := ts["id"]
			if !ok {
				return nil, fmt.Errorf("couldn't find power supply tag for host %s", host.Name)
			}
			idPlus, err := strconv.Atoi(id)
			if err != nil {
				slog.Errorf("couldn't conver it do integer for power supply id %s", id)
			}
			idPlus++
			ps := &PowerSupply{}
			fstatus, timestamp, err := s.Search.GetLast("hw.ps", ts.String(), false)
			ps.Amps, ps.AmpsLastUpdated, _ = s.Search.GetLast("hw.chassis.current.reading", opentsdb.TagSet{"host": host.Name, "id": fmt.Sprintf("PS%v", idPlus)}.String(), false)
			ps.Volts, ps.VoltsLastUpdated, _ = s.Search.GetLast("hw.chassis.volts.reading", opentsdb.TagSet{"host": host.Name, "name": fmt.Sprintf("PS%v_Voltage_%v", idPlus, idPlus)}.String(), false)
			if oldOrErr(timestamp, err) {
				continue
			}
			ps.Status = statusString(int64(fstatus), 0, "Ok", "Bad")
			host.Hardware.PowerSupplies[id] = ps
			for _, m := range hostMetadata {
				if m.Name != "psMeta" || m.Time.Before(utcNow().Add(-timeFilterAge)) || !m.Tags.Equal(ts) {
					continue
				}
				if val, ok := m.Value.(string); ok {
					err = json.Unmarshal([]byte(val), &ps)
					if err != nil {
						slog.Errorf("error unmarshalling power supply meta for host %s, while generating host api: %s", host.Name, err)
					} else {
						host.Hardware.PowerSupplies[id] = ps
					}
				}
			}
		}
		for _, ts := range hwBatteriesTags[host.Name] {
			id, ok := ts["id"]
			if !ok {
				slog.Errorf("couldn't find battery id tag %s for host %s", id, host.Name)
				continue
			}
			fstatus, timestamp, err := s.Search.GetLast("hw.storage.battery", ts.String(), false)
			if !oldOrErr(timestamp, err) {
				host.Hardware.Storage.Batteries[id] = &Battery{
					Status:            statusString(int64(fstatus), 0, "Ok", "Bad"),
					StatusLastUpdated: timestamp,
				}
			}
		}
		for _, ts := range hwBoardPowerTags[host.Name] {
			fstatus, timestamp, err := s.Search.GetLast("hw.chassis.power.reading", ts.String(), false)
			if !oldOrErr(timestamp, err) {
				host.Hardware.BoardPowerReading = &BoardPowerReading{
					Watts:            int64(fstatus),
					WattsLastUpdated: timestamp,
				}
			}
		}
		for _, ts := range hwPhysicalDiskTags[host.Name] {
			id, ok := ts["id"]
			if !ok {
				return nil, fmt.Errorf("couldn't find physical disk id tag for host %s", host.Name)
			}
			pd := &PhysicalDisk{}
			fstatus, timestamp, err := s.Search.GetLast("hw.storage.pdisk", ts.String(), false)
			if !oldOrErr(timestamp, err) {
				pd.Status = statusString(int64(fstatus), 0, "Ok", "Bad")
				pd.StatusLastUpdated = timestamp
				host.Hardware.Storage.PhysicalDisks[id] = pd
			}
			for _, m := range hostMetadata {
				if m.Name != "physicalDiskMeta" || m.Time.Before(utcNow().Add(-timeFilterAge)) || !m.Tags.Equal(ts) {
					continue
				}
				if val, ok := m.Value.(string); ok {
					err = json.Unmarshal([]byte(val), &pd)
					if err != nil {
						slog.Errorf("error unmarshalling addresses for host %s, interface %s while generating host api: %s", host.Name, m.Tags["iface"], err)
					} else {
						host.Hardware.Storage.PhysicalDisks[id] = pd
					}
				}
			}
		}
		for _, ts := range hwVirtualDiskTags[host.Name] {
			id, ok := ts["id"]
			if !ok {
				return nil, fmt.Errorf("couldn't find virtual disk id tag for host %s", host.Name)
			}
			fstatus, timestamp, err := s.Search.GetLast("hw.storage.vdisk", ts.String(), false)
			if !oldOrErr(timestamp, err) {
				host.Hardware.Storage.VirtualDisks[id] = &VirtualDisk{
					Status:            statusString(int64(fstatus), 0, "Ok", "Bad"),
					StatusLastUpdated: timestamp,
				}
			}
		}
		for _, ts := range hwControllersTags[host.Name] {
			id, ok := ts["id"]
			if !ok {
				return nil, fmt.Errorf("couldn't find controller id tag for host %s", host.Name)
			}
			fstatus, timestamp, err := s.Search.GetLast("hw.storage.controller", ts.String(), false)
			c := &Controller{}
			if !oldOrErr(timestamp, err) {
				c.Status = statusString(int64(fstatus), 0, "Ok", "Bad")
				c.StatusLastUpdated = timestamp
				host.Hardware.Storage.Controllers[id] = c
			}
			for _, m := range hostMetadata {
				if m.Name != "controllerMeta" || m.Time.Before(utcNow().Add(-timeFilterAge)) || !m.Tags.Equal(ts) {
					continue
				}
				if val, ok := m.Value.(string); ok {
					err = json.Unmarshal([]byte(val), &c)
					if err != nil {
						slog.Errorf("error unmarshalling controller meta for host %s: %s", host.Name, err)
					} else {
						host.Hardware.Storage.Controllers[id] = c
					}
				}
			}
		}
		for _, ts := range diskTags[host.Name] {
			disk, ok := ts["disk"]
			if !ok {
				return nil, fmt.Errorf("couldn't find disk tag for host %s", host.Name)
			}
			d := &Disk{}
			d.TotalBytes, d.StatsLastUpdated, err = s.Search.GetLastInt64("os.disk.fs.space_total", ts.String(), false)
			d.UsedBytes, _, _ = s.Search.GetLastInt64("os.disk.fs.space_used", ts.String(), false)
			if oldOrErr(d.StatsLastUpdated, err) {
				continue
			}
			host.Disks[disk] = d
			for _, m := range hostMetadata {
				if m.Name != "label" || m.Time.Before(utcNow().Add(-timeFilterAge)) || !m.Tags.Equal(ts) {
					continue
				}
				if label, ok := m.Value.(string); ok {
					host.Disks[disk].Label = label
					break
				}
			}
		}
		// Get CPU, Memory, Uptime
		var timestamp int64
		var cpu float64
		if cpu, timestamp, err = s.Search.GetLast("os.cpu", hostTagSet.String(), true); err != nil {
			cpu, timestamp, _ = s.Search.GetLast("cisco.cpu", hostTagSet.String(), false)
		}
		host.CPU.PercentUsed = cpu
		host.CPU.StatsLastUpdated = timestamp
		host.Memory.TotalBytes, host.Memory.StatsLastUpdated, _ = s.Search.GetLast("os.mem.total", hostTagSet.String(), false)
		host.Memory.UsedBytes, _, _ = s.Search.GetLast("os.mem.used", hostTagSet.String(), false)
		host.UptimeSeconds, _, _ = s.Search.GetLastInt64("os.system.uptime", hostTagSet.String(), false)
		for _, m := range hostMetadata {
			if m.Time.Before(utcNow().Add(-timeFilterAge)) {
				continue
			}
			var iface *HostInterface
			if name := m.Tags["iface"]; name != "" {
				if host.Interfaces[name] == nil {
					h := new(HostInterface)
					host.Interfaces[name] = h
				}
				iface = host.Interfaces[name]
			}
			if name := m.Tags["iname"]; name != "" && iface != nil {
				iface.Name = name
			}
			switch val := m.Value.(type) {
			case string:
				switch m.Name {
				case "addresses":
					if iface != nil {
						addresses := []string{}
						err = json.Unmarshal([]byte(val), &addresses)
						if err != nil {
							slog.Errorf("error unmarshalling addresses for host %s, interface %s while generating host api: %s", host.Name, m.Tags["iface"], err)
						}
						for _, address := range addresses {
							iface.IPAddresses = append(iface.IPAddresses, address)
						}
					}
				case "cdpCacheEntries":
					if iface != nil {
						var cdpCacheEntries CDPCacheEntries
						err = json.Unmarshal([]byte(val), &cdpCacheEntries)
						if err != nil {
							slog.Errorf("error unmarshalling cdpCacheEntries for host %s, interface %s while generating host api: %s", host.Name, m.Tags["iface"], err)
						} else {
							iface.CDPCacheEntries = cdpCacheEntries
						}
					}
				case "remoteMacs":
					if iface != nil {
						remoteMacs := []string{}
						err = json.Unmarshal([]byte(val), &remoteMacs)
						if err != nil {
							slog.Errorf("error unmarshalling remoteMacs for host %s, interface %s while generating host api: %s", host.Name, m.Tags["iface"], err)
						} else {
							iface.RemoteMacs = remoteMacs
						}
					}
				case "description", "alias":
					if iface != nil {
						iface.Description = val
					}
				case "dataStores":
					dataStores := []string{}
					err = json.Unmarshal([]byte(val), &dataStores)
					if err != nil {
						slog.Errorf("error unmarshalling datastores for host %s while generating host api: %s", host.Name, err)
					}
					for _, dataStore := range dataStores {
						tags := opentsdb.TagSet{"disk": dataStore}.String()
						total, totalTs, totalErr := s.Search.GetLastInt64("vsphere.disk.space_total", tags, false)
						used, usedTs, usedErr := s.Search.GetLastInt64("vsphere.disk.space_used", tags, false)
						if totalErr != nil || usedErr != nil || totalTs < 1 || usedTs < 1 {
							continue
						}
						host.Disks[dataStore] = &Disk{
							TotalBytes: total,
							UsedBytes:  used,
						}
					}
				case "mac":
					if iface != nil {
						iface.MAC = val
					}
				case "manufacturer":
					host.Manufacturer = val
				case "master":
					if iface != nil {
						iface.Master = val
					}
				case "memory":
					if name := m.Tags["name"]; name != "" {
						fstatus, timestamp, err := s.Search.GetLast("hw.chassis.memory", opentsdb.TagSet{"host": host.Name, "name": name}.String(), false)
						// Status code uses the severity function in collectors/dell_hw.go. That is a binary
						// state that is 0 for non-critical or Ok. Todo would be to update this with more
						// complete status codes when HW collector is refactored and we have something to
						// clean out addr entries from the tagset metadata db
						host.Hardware.Memory[name] = &MemoryModule{
							StatusLastUpdated: timestamp,
							Size:              val,
						}
						// Only set if we have a value
						if err == nil && timestamp > 0 {
							host.Hardware.Memory[name].Status = statusString(int64(fstatus), 0, "Ok", "Bad")
						}
					}
				case "hypervisor":
					host.VM = &VM{}
					host.VM.Host = val
					powerstate, timestamp, err := s.Search.GetLast("vsphere.guest.powered_state", opentsdb.TagSet{"guest": host.Name}.String(), false)
					if timestamp > 0 && err != nil {
						switch int64(powerstate) {
						case 0:
							host.VM.PowerState = "poweredOn"
						case 1:
							host.VM.PowerState = "poweredOff"
						case 2:
							host.VM.PowerState = "suspended"
						}
						host.VM.PowerStateLastUpdated = timestamp
					}
					if hostsHost, ok := hosts[val]; ok {
						hostsHost.Guests = append(hostsHost.Guests, host.Name)
					}
				case "model":
					host.Model = val
				case "name":
					if iface != nil {
						iface.Name = val
					}
				case "processor":
					if name := m.Tags["name"]; name != "" {
						host.CPU.Processors[name] = val
					}
				case "serialNumber":
					host.SerialNumber = val
				case "version":
					host.OS.Version = val
				case "versionCaption", "uname":
					host.OS.Caption = val
				}
			case float64:
				switch m.Name {
				case "speed":
					if iface != nil {
						iface.LinkSpeed = int64(val)
					}
				}
			}
		}
		GetIfaceBits := func(netType string, ifaceId string, iface *HostInterface, host string, tags []opentsdb.TagSet) error {
			metric := "os.net." + netType + ".bytes"
			if netType == "" {
				metric = "os.net.bytes"
			}
			for _, ts := range tags {
				if ts["iface"] != ifaceId {
					continue
				}
				dir, ok := ts["direction"]
				if !ok {
					continue
				}
				val, timestamp, _ := s.Search.GetLastInt64(metric, ts.String(), true)
				if dir == "in" {
					iface.Inbps = val * 8
				}
				if dir == "out" {
					iface.Outbps = val * 8
				}
				iface.StatsLastUpdated = timestamp
				iface.Type = netType
			}
			return nil
		}
		GetIfaceSpeed := func(netType string, ifaceId string, iface *HostInterface, host string, tags []opentsdb.TagSet) error {
			metric := "os.net." + netType + ".ifspeed"
			if netType == "" {
				metric = "os.net.ifspeed"
			}
			for _, ts := range tags {
				if ts["iface"] != ifaceId {
					continue
				}
				val, timestamp, err := s.Search.GetLastInt64(metric, ts.String(), false)
				if !oldOrErr(timestamp, err) {
					iface.LinkSpeed = val
				}
			}
			return nil
		}
		for ifaceId, iface := range host.Interfaces {
			if err := GetIfaceBits("", ifaceId, iface, host.Name, osNetBytesTags[host.Name]); err != nil {
				return nil, err
			}
			if err := GetIfaceBits("virtual", ifaceId, iface, host.Name, osNetVirtualBytesTags[host.Name]); err != nil {
				return nil, err
			}
			if err := GetIfaceBits("bond", ifaceId, iface, host.Name, osNetBondBytesTags[host.Name]); err != nil {
				return nil, err
			}
			if err := GetIfaceBits("tunnel", ifaceId, iface, host.Name, osNetTunnelBytesTags[host.Name]); err != nil {
				return nil, err
			}
			if err := GetIfaceBits("other", ifaceId, iface, host.Name, osNetOtherBytesTags[host.Name]); err != nil {
				return nil, err
			}
			if err := GetIfaceSpeed("", ifaceId, iface, host.Name, osNetIfSpeedTags[host.Name]); err != nil {
				return nil, err
			}
			if err := GetIfaceSpeed("virtual", ifaceId, iface, host.Name, osNetVirtualIfSpeedTags[host.Name]); err != nil {
				return nil, err
			}
			if err := GetIfaceSpeed("bond", ifaceId, iface, host.Name, osNetBondIfSpeedTags[host.Name]); err != nil {
				return nil, err
			}
			if err := GetIfaceSpeed("tunnel", ifaceId, iface, host.Name, osNetTunnelIfSpeedTags[host.Name]); err != nil {
				return nil, err
			}
			if err := GetIfaceSpeed("other", ifaceId, iface, host.Name, osNetOtherIfSpeedTags[host.Name]); err != nil {
				return nil, err
			}
		}
		host.Clean()
	}
	return hosts, nil
}

func statusString(val, goodVal int64, goodName, badName string) string {
	if val == goodVal {
		return goodName
	}
	return badName
}

func processHostIncidents(host *HostData, states States, silences SilenceTester) {
	for ak, state := range states {
		if stateHost, ok := state.AlertKey.Group()["host"]; !ok {
			continue
		} else if stateHost != host.Name {
			continue
		}
		silenced := silences(ak)
		is := IncidentStatus{
			IncidentID:         state.Id,
			Active:             state.IsActive(),
			AlertKey:           state.AlertKey,
			Status:             state.CurrentStatus,
			StatusTime:         state.Last().Time.Unix(),
			Subject:            state.Subject,
			Silenced:           silenced != nil,
			LastAbnormalStatus: state.LastAbnormalStatus,
			LastAbnormalTime:   state.LastAbnormalTime,
			NeedsAck:           state.NeedAck,
		}
		host.OpenIncidents = append(host.OpenIncidents, is)
	}
}

// Cisco Discovery Protocol
type CDPCacheEntry struct {
	DeviceID   string
	DevicePort string
}

type CDPCacheEntries []CDPCacheEntry

type HostInterface struct {
	Description      string          `json:",omitempty"`
	IPAddresses      []string        `json:",omitempty"`
	RemoteMacs       []string        `json:",omitempty"`
	CDPCacheEntries  CDPCacheEntries `json:",omitempty"`
	Inbps            int64
	LinkSpeed        int64  `json:",omitempty"`
	MAC              string `json:",omitempty"`
	Master           string `json:",omitempty"`
	Name             string `json:",omitempty"`
	Outbps           int64
	StatsLastUpdated int64
	Type             string
}

type Disk struct {
	UsedBytes        int64
	TotalBytes       int64
	Label            string `json:",omitempty"`
	StatsLastUpdated int64
}

type MemoryModule struct {
	// Maybe this should be a bool but that might be limiting
	Status            string
	StatusLastUpdated int64
	Size              string
}

type ChassisComponent struct {
	Status            string
	StatusLastUpdated int64
}

type PowerSupply struct {
	Status                     string
	StatusLastUpdated          int64
	Amps                       float64
	AmpsLastUpdated            int64
	Volts                      float64
	VoltsLastUpdated           int64
	metadata.HWPowerSupplyMeta //Should be renamed to Meta
}

type PhysicalDisk struct {
	Status            string
	StatusLastUpdated int64
	metadata.HWDiskMeta
}

type VirtualDisk struct {
	Status            string
	StatusLastUpdated int64
}

type Controller struct {
	Status            string
	StatusLastUpdated int64
	metadata.HWControllerMeta
}

type Temp struct {
	Celsius            float64
	Status             string
	StatusLastUpdated  int64
	CelsiusLastUpdated int64
}

type ICMPData struct {
	TimedOut               bool
	TimedOutLastUpdated    int64
	DNSResolved            bool
	DNSResolvedLastUpdated int64
	RTTMS                  float64
	RTTLastUpdated         int64
}

func newHostData() *HostData {
	hd := &HostData{}
	hd.CPU.Processors = make(map[string]string)
	hd.Interfaces = make(map[string]*HostInterface)
	hd.Disks = make(map[string]*Disk)
	hd.Services = make(map[string]*ServiceStatus)
	hd.Processes = make(map[string]*Process)
	hd.ICMPData = make(map[string]*ICMPData)
	hd.Hardware = &Hardware{}
	hd.Hardware.ChassisComponents = make(map[string]*ChassisComponent)
	hd.Hardware.Temps = make(map[string]*Temp)
	hd.Hardware.PowerSupplies = make(map[string]*PowerSupply)
	hd.Hardware.Memory = make(map[string]*MemoryModule)
	hd.Hardware.Storage.PhysicalDisks = make(map[string]*PhysicalDisk)
	hd.Hardware.Storage.VirtualDisks = make(map[string]*VirtualDisk)
	hd.Hardware.Storage.Controllers = make(map[string]*Controller)
	hd.Hardware.Storage.Batteries = make(map[string]*Battery)
	return hd
}

// Clean sets certain maps to nil so they don't get marshalled in JSON
// The logic is to remove ones that monitored devices might lack, such
// as hardware information
func (hd *HostData) Clean() {
	if len(hd.CPU.Processors) == 0 {
		hd.CPU.Processors = nil
	}
	if len(hd.Disks) == 0 {
		hd.Disks = nil
	}
	if len(hd.Services) == 0 {
		hd.Services = nil
	}
	if len(hd.Processes) == 0 {
		hd.Processes = nil
	}
	hwLen := len(hd.Hardware.ChassisComponents) +
		len(hd.Hardware.Memory) +
		len(hd.Hardware.Storage.PhysicalDisks) +
		len(hd.Hardware.Storage.VirtualDisks) +
		len(hd.Hardware.Storage.Controllers) +
		len(hd.Hardware.PowerSupplies) +
		len(hd.Hardware.Temps) +
		len(hd.Hardware.Storage.Batteries)
	if hwLen == 0 {
		hd.Hardware = nil
	}
}

type Hardware struct {
	Memory            map[string]*MemoryModule     `json:",omitempty"`
	ChassisComponents map[string]*ChassisComponent `json:",omitempty"`
	Storage           struct {
		Controllers   map[string]*Controller   `json:",omitempty"`
		PhysicalDisks map[string]*PhysicalDisk `json:",omitempty"`
		VirtualDisks  map[string]*VirtualDisk  `json:",omitempty"`
		Batteries     map[string]*Battery
	}
	Temps             map[string]*Temp
	PowerSupplies     map[string]*PowerSupply `json:",omitempty"`
	BoardPowerReading *BoardPowerReading
}

type BoardPowerReading struct {
	Watts            int64
	WattsLastUpdated int64
}

type VM struct {
	Host                  string `json:",omitempty"`
	PowerState            string `json:",omitempty"`
	PowerStateLastUpdated int64  `json:",omitempty"`
}

type Battery struct {
	Status            string
	StatusLastUpdated int64
}

type ServiceStatus struct {
	Running            bool
	RunningLastUpdated int64
}

type Process struct {
	CPUPercentUsed              float64
	CPUPercentLastUpdated       int64
	UsedRealBytes               int64
	UsedRealBytesLastUpdated    int64
	UsedVirtualBytes            int64
	UsedVirtualBytesLastUpdated int64
	Count                       int64
	CountLastUpdated            int64
}

type HostData struct {
	CPU struct {
		Logical          int64 `json:",omitempty"`
		Physical         int64 `json:",omitempty"`
		PercentUsed      float64
		StatsLastUpdated int64
		Processors       map[string]string `json:",omitempty"`
	}
	ICMPData      map[string]*ICMPData
	Disks         map[string]*Disk
	OpenIncidents []IncidentStatus
	Interfaces    map[string]*HostInterface
	UptimeSeconds int64     `json:",omitempty"`
	Manufacturer  string    `json:",omitempty"`
	Hardware      *Hardware `json:",omitempty"`
	Memory        struct {
		TotalBytes       float64
		UsedBytes        float64
		StatsLastUpdated int64
	}
	Processes map[string]*Process       `json:",omitempty"`
	Services  map[string]*ServiceStatus `json:",omitempty"`
	Model     string                    `json:",omitempty"`
	Name      string                    `json:",omitempty"`
	OS        struct {
		Caption string `json:",omitempty"`
		Version string `json:",omitempty"`
	}
	SerialNumber string   `json:",omitempty"`
	VM           *VM      `json:",omitempty"`
	Guests       []string `json:",omitempty"`
}
