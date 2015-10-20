package sched // import "bosun.org/cmd/bosun/sched"
import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"bosun.org/cmd/bosun/expr"
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
	states := s.GetOpenStates()
	silences := s.Silenced()
	// These are all fetched by metric since that is how we store it in redis
	// so this makes for the fastest response
	osNetBytesTags, err := s.Search.FilteredTagSets("os.net.bytes", nil)
	if err != nil {
		return nil, err
	}
	osNetVirtualBytesTags, err := s.Search.FilteredTagSets("os.net.virtual.bytes", nil)
	if err != nil {
		return nil, err
	}
	osNetBondBytesTags, err := s.Search.FilteredTagSets("os.net.bond.bytes", nil)
	if err != nil {
		return nil, err
	}
	osNetTunnelBytesTags, err := s.Search.FilteredTagSets("os.net.tunnel.bytes", nil)
	if err != nil {
		return nil, err
	}
	osNetOtherBytesTags, err := s.Search.FilteredTagSets("os.net.other.bytes", nil)
	if err != nil {
		return nil, err
	}
	osNetIfSpeedTags, err := s.Search.FilteredTagSets("os.net.ifspeed", nil)
	if err != nil {
		return nil, err
	}
	osNetVirtualIfSpeedTags, err := s.Search.FilteredTagSets("os.net.virtual.ifspeed", nil)
	if err != nil {
		return nil, err
	}
	osNetBondIfSpeedTags, err := s.Search.FilteredTagSets("os.net.bond.ifspeed", nil)
	if err != nil {
		return nil, err
	}
	osNetTunnelIfSpeedTags, err := s.Search.FilteredTagSets("os.net.tunnel.ifspeed", nil)
	if err != nil {
		return nil, err
	}
	osNetOtherIfSpeedTags, err := s.Search.FilteredTagSets("os.net.other.ifspeed", nil)
	if err != nil {
		return nil, err
	}
	hwChassisTags, err := s.Search.FilteredTagSets("hw.chassis", nil)
	if err != nil {
		return nil, err
	}
	hwPhysicalDiskTags, err := s.Search.FilteredTagSets("hw.storage.pdisk", nil)
	if err != nil {
		return nil, err
	}
	hwVirtualDiskTags, err := s.Search.FilteredTagSets("hw.storage.vdisk", nil)
	if err != nil {
		return nil, err
	}
	hwControllersTags, err := s.Search.FilteredTagSets("hw.storage.controller", nil)
	if err != nil {
		return nil, err
	}
	hwBatteriesTags, err := s.Search.FilteredTagSets("hw.storage.battery", nil)
	if err != nil {
		return nil, err
	}
	hwPowerSuppliesTags, err := s.Search.FilteredTagSets("hw.ps", nil)
	if err != nil {
		return nil, err
	}
	hwTempsTags, err := s.Search.FilteredTagSets("hw.chassis.temps.reading", nil)
	if err != nil {
		return nil, err
	}
	hwBoardPowerTags, err := s.Search.FilteredTagSets("hw.chassis.power.reading", nil)
	if err != nil {
		return nil, err
	}
	diskTags, err := s.Search.FilteredTagSets("os.disk.fs.space_total", nil)
	if err != nil {
		return nil, err
	}
	// Will make the assumption that the metric bosun.ping.timeout, resolved, and rtt
	// all share the same tagset
	icmpTimeOutTags, err := s.Search.FilteredTagSets("bosun.ping.timeout", nil)
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
		for _, ts := range icmpTimeOutTags {
			if ts["dst_host"] != host.Name {
				continue
			}
			// The host tag represents the polling source for these set of metrics
			source, ok := ts["host"]
			if !ok {
				slog.Errorf("couldn't find source tag for icmp data for host %s", host.Name)
			}
			// 1 Means it timed out
			timeout, timestamp, err := s.Search.GetLast("bosun.ping.timeout", ts.String(), false)
			if err != nil && !(timestamp > 0) {
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
				RTTMS:          rtt,
				RTTLastUpdated: rttTimestamp,
			}

		}
		// Process Hardware Chassis States
		for _, ts := range hwChassisTags {
			if ts["host"] != host.Name {
				continue
			}
			component, ok := ts["component"]
			if !ok {
				return nil, fmt.Errorf("couldn't find component tag for host %s", host.Name)
			}
			fstatus, timestamp, err := s.Search.GetLast("hw.chassis", ts.String(), false)
			status := "Bad"
			if fstatus == 0 {
				status = "Ok"
			}
			if err == nil && timestamp > 0 {
				host.Hardware.ChassisComponents[component] = &ChassisComponent{
					Status:            status,
					StatusLastUpdated: timestamp,
				}
			}
		}
		for _, ts := range hwTempsTags {
			if ts["host"] != host.Name {
				continue
			}
			name, ok := ts["name"]
			if !ok {
				slog.Errorf("couldn't find name tag %s for host %s", host.Name, name)
			}
			tStatus, timestamp, err := s.Search.GetLast("hw.chassis.temps", ts.String(), false)
			celsius, rTimestamp, err := s.Search.GetLast("hw.chassis.temps.reading", ts.String(), false)
			status := "Bad"
			if tStatus == 0 {
				status = "Ok"
			}
			if err == nil && timestamp > 0 {
				host.Hardware.Temps[name] = &Temp{
					Celsius:            celsius,
					Status:             status,
					StatusLastUpdated:  timestamp,
					CelsiusLastUpdated: rTimestamp,
				}
			}
		}
		for _, ts := range hwPowerSuppliesTags {
			if ts["host"] != host.Name {
				continue
			}
			id, ok := ts["id"]
			if !ok {
				return nil, fmt.Errorf("couldn't find power supply tag for host %s", host.Name)
			}
			idPlus, err := strconv.Atoi(id)
			if err != nil {
				slog.Errorf("couldn't conver it do integer for power supply id %s", id)
			}
			idPlus++
			fstatus, timestamp, err := s.Search.GetLast("hw.ps", ts.String(), false)
			status := "Bad"
			if fstatus == 0 {
				status = "Ok"
			}
			current, currentTimestamp, _ := s.Search.GetLast("hw.chassis.current.reading", opentsdb.TagSet{"host": host.Name, "id": fmt.Sprintf("PS%v", idPlus)}.String(), false)
			volts, voltsTimestamp, _ := s.Search.GetLast("hw.chassis.volts.reading", opentsdb.TagSet{"host": host.Name, "name": fmt.Sprintf("PS%v_Voltage_%v", idPlus, idPlus)}.String(), false)
			ps := &PowerSupply{}
			if err == nil && timestamp > 0 {
				ps.Status = status
				ps.StatusLastUpdated = timestamp
				ps.Amps = current
				ps.AmpsLastUpdated = currentTimestamp
				ps.Volts = volts
				ps.VoltsLastUpdated = voltsTimestamp
				host.Hardware.PowerSupplies[id] = ps
			}
			for _, m := range hostMetadata {
				if m.Time.Before(time.Now().Add(-timeFilterAge)) {
					continue
				}
				if !m.Tags.Equal(ts) {
					continue
				}
				switch val := m.Value.(type) {
				case string:
					switch m.Name {
					case "psMeta":
						err = json.Unmarshal([]byte(val), &ps)
						if err != nil {
							slog.Errorf("error unmarshalling power supply meta for host %s, while generating host api: %s", host.Name, err)
						} else {
							host.Hardware.PowerSupplies[id] = ps
						}
					}
				}
			}
		}
		for _, ts := range hwBatteriesTags {
			if ts["host"] != host.Name {
				continue
			}
			id, ok := ts["id"]
			if !ok {
				slog.Errorf("couldn't find battery id tag %s for host %s", host.Name, id)
				continue
			}
			fstatus, timestamp, err := s.Search.GetLast("hw.storage.battery", ts.String(), false)
			status := "Bad"
			if fstatus == 0 {
				status = "Ok"
			}
			if err == nil && timestamp > 0 {
				host.Hardware.Storage.Batteries[id] = &Battery{
					Status:            status,
					StatusLastUpdated: timestamp,
				}
			}
		}
		for _, ts := range hwBoardPowerTags {
			if ts["host"] != host.Name {
				continue
			}
			fstatus, timestamp, err := s.Search.GetLast("hw.chassis.power.reading", ts.String(), false)
			if err == nil && timestamp > 0 {
				host.Hardware.BoardPowerReading = &BoardPowerReading{
					Watts:            int64(fstatus),
					WattsLastUpdated: timestamp,
				}
			}
		}
		for _, ts := range hwPhysicalDiskTags {
			if ts["host"] != host.Name {
				continue
			}
			id, ok := ts["id"]
			if !ok {
				return nil, fmt.Errorf("couldn't find physical disk id tag for host %s", host.Name)
			}
			fstatus, timestamp, err := s.Search.GetLast("hw.storage.pdisk", ts.String(), false)
			status := "Bad"
			if fstatus == 0 {
				status = "Ok"
			}
			pd := &PhysicalDisk{}
			if err == nil && timestamp > 0 {
				pd.Status = status
				pd.StatusLastUpdated = timestamp
				host.Hardware.Storage.PhysicalDisks[id] = pd
			}
			for _, m := range hostMetadata {
				if m.Time.Before(time.Now().Add(-timeFilterAge)) {
					continue
				}
				if !m.Tags.Equal(ts) {
					continue
				}
				switch val := m.Value.(type) {
				case string:
					switch m.Name {
					case "physicalDiskMeta":
						err = json.Unmarshal([]byte(val), &pd)
						if err != nil {
							slog.Errorf("error unmarshalling addresses for host %s, interface %s while generating host api: %s", host.Name, m.Tags["iface"], err)
						} else {
							host.Hardware.Storage.PhysicalDisks[id] = pd
						}
					}
				}
			}
		}
		for _, ts := range hwVirtualDiskTags {
			if ts["host"] != host.Name {
				continue
			}
			id, ok := ts["id"]
			if !ok {
				return nil, fmt.Errorf("couldn't find virtual disk id tag for host %s", host.Name)
			}
			fstatus, timestamp, err := s.Search.GetLast("hw.storage.vdisk", ts.String(), false)
			status := "Bad"
			if fstatus == 0 {
				status = "Ok"
			}
			if err == nil && timestamp > 0 {
				host.Hardware.Storage.VirtualDisks[id] = &VirtualDisk{
					Status:            status,
					StatusLastUpdated: timestamp,
				}
			}
		}
		for _, ts := range hwControllersTags {
			if ts["host"] != host.Name {
				continue
			}
			id, ok := ts["id"]
			if !ok {
				return nil, fmt.Errorf("couldn't find controller id tag for host %s", host.Name)
			}
			fstatus, timestamp, err := s.Search.GetLast("hw.storage.controller", ts.String(), false)
			status := "Bad"
			if fstatus == 0 {
				status = "Ok"
			}
			c := &Controller{}
			if err == nil && timestamp > 0 {
				c.Status = status
				c.StatusLastUpdated = timestamp
				host.Hardware.Storage.Controllers[id] = c
			}
			for _, m := range hostMetadata {
				if m.Time.Before(time.Now().Add(-timeFilterAge)) {
					continue
				}
				if !m.Tags.Equal(ts) {
					continue
				}
				switch val := m.Value.(type) {
				case string:
					switch m.Name {
					case "controllerMeta":
						err = json.Unmarshal([]byte(val), &c)
						if err != nil {
							slog.Errorf("error unmarshalling controller meta for host %s %s", host.Name, err)
						} else {
							host.Hardware.Storage.Controllers[id] = c
						}
					}
				}
			}
		}
		for _, ts := range diskTags {
			if ts["host"] != host.Name {
				continue
			}
			disk, ok := ts["disk"]
			if !ok {
				return nil, fmt.Errorf("couldn't find disk tag for host %s", host.Name)
			}
			total, timestamp, _ := s.Search.GetLast("os.disk.fs.space_total", ts.String(), false)
			used, _, _ := s.Search.GetLast("os.disk.fs.space_used", ts.String(), false)
			host.Disks[disk] = &Disk{
				TotalBytes:       total,
				UsedBytes:        used,
				StatsLastUpdated: timestamp,
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
		host.Memory.TotalBytes, timestamp, _ = s.Search.GetLast("os.mem.total", hostTagSet.String(), false)
		host.Memory.UsedBytes, _, _ = s.Search.GetLast("os.mem.used", hostTagSet.String(), false)
		host.Memory.StatsLastUpdated = timestamp
		var uptime float64
		uptime, timestamp, err = s.Search.GetLast("os.system.uptime", hostTagSet.String(), false)
		if err == nil && timestamp > 0 {
			host.UptimeSeconds = int64(uptime)
		}
		for _, m := range hostMetadata {
			if m.Time.Before(time.Now().Add(-timeFilterAge)) {
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
			if name := m.Tags["iname"]; name != "" {
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
				case "description", "alias":
					if iface != nil {
						iface.Description = val
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
						statusCode, timestamp, err := s.Search.GetLast("hw.chassis.memory", opentsdb.TagSet{"host": host.Name, "name": name}.String(), false)
						// Status code uses the severity function in collectors/dell_hw.go. That is a binary
						// state that is 0 for non-critical or Ok. Todo would be to update this with more
						// complete status codes when HW collector is refactored and we have something to
						// clean out addr entries from the tagset metadata db
						host.Hardware.Memory[name] = &MemoryModule{
							StatusLastUpdated: timestamp,
							Size:              val,
						}
						status := "Bad"
						if statusCode == 0 {
							status = "Ok"
						}
						// Only set if we have a value
						if err == nil && timestamp > 0 {
							host.Hardware.Memory[name].Status = status
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
					//Is this safe?
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
				if ts["iface"] != ifaceId || ts["host"] != host {
					continue
				}
				dir, ok := ts["direction"]
				if !ok {
					continue
				}
				val, timestamp, _ := s.Search.GetLast(metric, ts.String(), true)
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
				if ts["iface"] != ifaceId || ts["host"] != host {
					continue
				}
				val, timestamp, err := s.Search.GetLast(metric, ts.String(), false)
				if err == nil && timestamp > 0 {
					iface.LinkSpeed = int64(val)
				}
			}
			return nil
		}
		for ifaceId, iface := range host.Interfaces {
			if err := GetIfaceBits("", ifaceId, iface, host.Name, osNetBytesTags); err != nil {
				return nil, err
			}
			if err := GetIfaceBits("virtual", ifaceId, iface, host.Name, osNetVirtualBytesTags); err != nil {
				return nil, err
			}
			if err := GetIfaceBits("bond", ifaceId, iface, host.Name, osNetBondBytesTags); err != nil {
				return nil, err
			}
			if err := GetIfaceBits("tunnel", ifaceId, iface, host.Name, osNetTunnelBytesTags); err != nil {
				return nil, err
			}
			if err := GetIfaceBits("other", ifaceId, iface, host.Name, osNetOtherBytesTags); err != nil {
				return nil, err
			}
			if err := GetIfaceSpeed("", ifaceId, iface, host.Name, osNetIfSpeedTags); err != nil {
				return nil, err
			}
			if err := GetIfaceSpeed("virtual", ifaceId, iface, host.Name, osNetVirtualIfSpeedTags); err != nil {
				return nil, err
			}
			if err := GetIfaceSpeed("bond", ifaceId, iface, host.Name, osNetBondIfSpeedTags); err != nil {
				return nil, err
			}
			if err := GetIfaceSpeed("tunnel", ifaceId, iface, host.Name, osNetTunnelIfSpeedTags); err != nil {
				return nil, err
			}
			if err := GetIfaceSpeed("other", ifaceId, iface, host.Name, osNetOtherIfSpeedTags); err != nil {
				return nil, err
			}
		}
		host.Clean()
	}
	return hosts, nil
}

func processHostIncidents(host *HostData, states States, silences map[expr.AlertKey]Silence) {
	for ak, state := range states {
		if stateHost, ok := state.Group["host"]; !ok {
			continue
		} else if stateHost != host.Name {
			continue
		}
		_, silenced := silences[ak]
		is := IncidentStatus{
			IncidentID:         state.Last().IncidentId,
			Active:             state.IsActive(),
			AlertKey:           state.AlertKey(),
			Status:             state.Status(),
			StatusTime:         state.Last().Time.Unix(),
			Subject:            state.Subject,
			Silenced:           silenced,
			LastAbnormalStatus: state.AbnormalStatus(),
			LastAbnormalTime:   state.AbnormalEvent().Time.Unix(),
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
	Inbps            float64
	LinkSpeed        int64  `json:",omitempty"`
	MAC              string `json:",omitempty"`
	Master           string `json:",omitempty"`
	Name             string `json:",omitempty"`
	Outbps           float64
	StatsLastUpdated int64
	Type             string
}

type Disk struct {
	UsedBytes        float64
	TotalBytes       float64
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
	Model string `json:",omitempty"`
	Name  string `json:",omitempty"`
	OS    struct {
		Caption string `json:",omitempty"`
		Version string `json:",omitempty"`
	}
	SerialNumber string   `json:",omitempty"`
	VM           *VM      `json:",omitempty"`
	Guests       []string `json:",omitempty"`
}
