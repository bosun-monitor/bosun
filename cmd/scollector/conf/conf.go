// Package conf contains all of the configuration structs for scollector.
package conf // import "bosun.org/cmd/scollector/conf"

import (
	"bosun.org/opentsdb"
)

type Conf struct {
	// Host is the OpenTSDB or Bosun host to send data.
	Host string
	// FullHost enables full hostnames: doesn't truncate to first ".".
	FullHost bool
	// ColDir is the external collectors directory.
	ColDir string
	// Tags are added to every datapoint. If a collector specifies the same tag
	// key, this one will be overwritten. The host tag is not supported.
	Tags opentsdb.TagSet
	// Hostname overrides the system hostname.
	Hostname string
	// DisableSelf disables sending of scollector self metrics.
	DisableSelf bool
	// Freq is the default frequency in seconds for most collectors.
	Freq int
	// Filter filters collectors matching these terms.
	Filter []string

	// KeepalivedCommunity, if not empty, enables the Keepalived collector with
	// the specified community.
	KeepalivedCommunity string

	HAProxy       []HAProxy
	SNMP          []SNMP
	ICMP          []ICMP
	Vsphere       []Vsphere
	AWS           []AWS
	Process       []ProcessParams
	ProcessDotNet []ProcessDotNet
	HTTPUnit      []HTTPUnit
}

type HAProxy struct {
	User      string
	Password  string
	Instances []HAProxyInstance
}

type HAProxyInstance struct {
	Tier string
	URL  string
}

type ICMP struct {
	Host string
}

type Vsphere struct {
	Host     string
	User     string
	Password string
}

type AWS struct {
	AccessKey string
	SecretKey string
	Region    string
}

type SNMP struct {
	Community string
	Host      string
}

type ProcessDotNet struct {
	Name string
}

type HTTPUnit struct {
	TOML  string
	Hiera string
}
