package collectors

import (
	"fmt"
	"reflect"
	"strings"

	"bosun.org/_third_party/github.com/mjibson/snmp"

	"bosun.org/metadata"
	"bosun.org/opentsdb"
)

type VRRPInstanceEntry struct {
	VInstanceIndex             int64
	VInstanceName              string
	VInstanceVirtualRouterId   int64
	VInstanceState             int64
	VInstanceInitialState      int64
	VInstanceWantedState       int64
	VInstanceBasePriority      int64
	VInstanceEffectivePriority int64
	VInstanceVipsStatus        int64
	VInstancePrimaryInterface  string
	VInstanceTrackPrimaryIf    int64
	VInstanceAdvertisementsInt int64
	VInstancePreempt           int64
	VInstancePreemptDelay      int64
	VInstanceAuthType          int64
	VInstanceLvsSyncDaemon     int64
	VInstanceLvsSyncInterface  string
	VInstanceSyncGroup         string
	VInstanceGarpDelay         int64
	VInstanceSmtpAlert         int64
	VInstanceNotifyExec        int64
	VInstanceScriptMaster      string
	VInstanceScriptBackup      string
	VInstanceScriptFault       string
	VInstanceScriptStop        string
	VInstanceScript            string
	VInstanceAccept            int64
}

const (
	VRRPInstanceTable = ".1.3.6.1.4.1.9586.100.5.2.3.1"
	VRRPAddressTable  = ".1.3.6.1.4.1.9586.100.5.2.6.1"
)

const (
	descVRRPState              = "VRRP Can be in one of the following states: init(0), backup(1), master(2), fault(3), unknown(4)."
	descVRRPVipsStatus         = "Indicates if all the VIPs of this VRRP instance are enabled."
	descVRRPBasePriority       = "Base priority (as defined in the configuration file) for this VRRP instance. This value can be modified to force the virtual router instance to become backup or master."
	descVRRPEffectivePriority  = "Effective priority for this VRRP instance. Status of interfaces and script results are used to compute this value from the base priority."
	descVRRPAddressStatus      = "Indicates if the IP address is set or not."
	descVRRPAddressAdvertising = "Indicates if the IP address is being advertised or not."
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_snmp_keepalived_vrrp_instances})
}

func c_snmp_keepalived_vrrp_instances() (opentsdb.MultiDataPoint, error) {
	if KeepalivedCommunity == "" {
		return nil, nil
	}
	var md opentsdb.MultiDataPoint
	entries := make(map[int]*VRRPInstanceEntry)
	rows, err := snmp.Walk("localhost", KeepalivedCommunity, VRRPInstanceTable)
	if err != nil {
		return nil, nil
	}
	for rows.Next() {
		var a interface{}
		i, err := rows.Scan(&a)
		if err != nil {
			return nil, err
		}
		id, ok := i.([]int)
		if !ok || len(id) != 2 {
			return nil, fmt.Errorf("unexpected type for snmp keepalived index")
		}
		entry, ok := entries[id[1]]
		if !ok {
			entries[id[1]] = &VRRPInstanceEntry{}
			entry = entries[id[1]]
		}
		s := reflect.ValueOf(entry)
		nFields := reflect.ValueOf(*entry).NumField()
		if id[0]+1 > nFields {
			return nil, fmt.Errorf("unexpected number of fields for snmp keepalived VRRPInstanceTable")
		}
		v := s.Elem().Field(id[0] - 1)
		switch t := a.(type) {
		case int64:
			v.SetInt(t)
		case []uint8:
			v.SetString(string(t))
		}
	}
	for _, entry := range entries {
		ts := opentsdb.TagSet{"instance_name": entry.VInstanceName, "instance_id": fmt.Sprint(entry.VInstanceVirtualRouterId)}
		Add(&md, "keepalived.vrrp.state", entry.VInstanceState, ts, metadata.Gauge, metadata.StatusCode, descVRRPState)
		Add(&md, "keepalived.vrrp.wanted_state", entry.VInstanceWantedState, ts, metadata.Gauge, metadata.StatusCode, descVRRPState)
		Add(&md, "keepalived.vrrp.vips_status", entry.VInstanceVipsStatus, ts, metadata.Gauge, metadata.StatusCode, descVRRPVipsStatus)
		Add(&md, "keepalived.vrrp.base_priority", entry.VInstanceBasePriority, ts, metadata.Gauge, metadata.Priority, descVRRPBasePriority)
		Add(&md, "keepalived.vrrp.effective_priority", entry.VInstanceEffectivePriority, ts, metadata.Gauge, metadata.Priority, descVRRPEffectivePriority)
	}
	if err := keepalived_vrrp_addresses(&md, entries); err != nil {
		return nil, err
	}
	return md, nil
}

type VRRPAddressEntry struct {
	VRRPAddressIndex       int64
	VRRPAddressType        int64
	VRRPAddressValue       string `snmp:"octet"`
	VRRPAddressBroadcast   string `snmp:"octet"`
	VRRPAddressMask        int64
	VRRPAddressScope       int64
	VRRPAddressIfIndex     int64
	VRRPAddressIfName      string
	VRRPAddressIfAlias     string
	VRRPAddressStatus      int64
	VRRPAddressAdvertising int64
}

// Field (i.e Vrrp address type), Instance Index, Identifer

func keepalived_vrrp_addresses(md *opentsdb.MultiDataPoint, instances map[int]*VRRPInstanceEntry) error {
	entries := make(map[int]map[int]*VRRPAddressEntry)
	rows, err := snmp.Walk("localhost", KeepalivedCommunity, VRRPAddressTable)
	if err != nil {
		return nil
	}
	for rows.Next() {
		var a interface{}
		i, err := rows.Scan(&a)
		if err != nil {
			return err
		}
		id, ok := i.([]int)
		if !ok || len(id) != 3 {
			return fmt.Errorf("unexpected type for snmp keepalived index")
		}
		if _, ok := entries[id[1]]; !ok {
			entries[id[1]] = make(map[int]*VRRPAddressEntry)
		}
		entry, ok := entries[id[1]][id[2]]
		if !ok {
			entries[id[1]][id[2]] = &VRRPAddressEntry{}
			entry = entries[id[1]][id[2]]
		}
		s := reflect.ValueOf(entry)
		nFields := reflect.ValueOf(*entry).NumField()
		nonPointerType := reflect.ValueOf(*entry).Type()
		if id[0]-1 > nFields {
			return fmt.Errorf("unexpected number of fields for snmp keepalived VRRPAddressTable")
		}
		v := s.Elem().Field(id[0] - 1)
		switch t := a.(type) {
		case int64:
			v.SetInt(t)
		case []uint8:
			if nonPointerType.Kind() == reflect.Struct && nonPointerType.Field(id[0]-1).Tag.Get("snmp") == "octet" {
				var s []string
				for _, runeValue := range t {
					s = append(s, fmt.Sprintf("%v", runeValue))
				}
				v.SetString(strings.Join(s, "."))
			} else {
				v.SetString(string(t))
			}
		}
	}
	for instance_id, instance := range instances {
		for _, entry := range entries[instance_id] {
			ts := opentsdb.TagSet{
				"instance_name": instance.VInstanceName,
				"instance_id":   fmt.Sprint(instance.VInstanceVirtualRouterId),
				"address":       entry.VRRPAddressValue}
			Add(md, "keepalived.vrrp.address_status", entry.VRRPAddressStatus-1, ts, metadata.Gauge, metadata.Bool, descVRRPAddressStatus)
			Add(md, "keepalived.vrrp.address_advertising", entry.VRRPAddressAdvertising-1, ts, metadata.Gauge, metadata.Bool, descVRRPAddressStatus)
		}
	}
	return nil
}
