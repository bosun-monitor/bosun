// Package conf contains all of the configuration structs for scollector.
package conf // import "bosun.org/cmd/scollector/conf"

import (
	"bosun.org/opentsdb"
)

type Conf struct {
	// Host is the OpenTSDB or Bosun host to send data.
	Host string `json:",omitempty"`
	// FullHost enables full hostnames: doesn't truncate to first ".".
	FullHost bool `json:",omitempty"`
	// ColDir is the external collectors directory.
	ColDir string `json:",omitempty"`
	// Tags are added to every datapoint. If a collector specifies the same tag
	// key, this one will be overwritten. The host tag is not supported.
	Tags opentsdb.TagSet `json:",omitempty"`
	// Hostname overrides the system hostname.
	Hostname string `json:",omitempty"`
	// DisableSelf disables sending of scollector self metrics.
	DisableSelf bool `json:",omitempty"`
	// Freq is the default frequency in seconds for most collectors.
	Freq int
	// BatchSize is the number of metrics that will be sent in each batch.
	BatchSize int
	// Filter filters collectors matching these terms.
	Filter []string `json:",omitempty"`
	// PProf is an IP:Port binding to be used for debugging with pprof package.
	// Examples: localhost:6060 for loopback or :6060 for all IP addresses.
	PProf string `json:",omitempty"`

	// KeepalivedCommunity, if not empty, enables the Keepalived collector with
	// the specified community.
	KeepalivedCommunity string `json:",omitempty"`

	HAProxy       []HAProxy       `json:",omitempty"`
	SNMP          []SNMP          `json:",omitempty"`
	MIBS          map[string]MIB  `json:",omitempty"`
	ICMP          []ICMP          `json:",omitempty"`
	Vsphere       []Vsphere       `json:",omitempty"`
	AWS           []AWS           `json:",omitempty"`
	Process       []ProcessParams `json:",omitempty"`
	ProcessDotNet []ProcessDotNet `json:",omitempty"`
	HTTPUnit      []HTTPUnit      `json:",omitempty"`
	Riak          []Riak          `json:",omitempty"`
	Github        []Github        `json:",omitempty"`
	// ElasticIndexFilters takes regular expressions and excludes indicies that
	// match those filters from being monitored for metrics in the elastic.indices
	// namespace
	ElasticIndexFilters []string   `json:",omitempty"`
	RabbitMQ            []RabbitMQ `json:",omitempty"`
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
	MIBs      []string
}

type MIB struct {
	BaseOid string
	Metrics []MIBMetric `json:",omitempty"` // single key metrics
	Trees   []MIBTree   `json:",omitempty"` // tagged array metrics
}

type MIBMetric struct {
	Metric      string
	Oid         string
	Unit        string `json:",omitempty"` // metadata unit
	RateType    string `json:",omitempty"` // defaults to gauge
	Description string `json:",omitempty"`
	FallbackOid string `json:",omitempty"` // Oid to try if main one doesn't work. Used in cisco where different models use different oids
	Tags        string `json:",omitempty"` // static tags to populate for this metric. "direction=in"
}

type MIBTag struct {
	Key string
	Oid string // If present will load from this oid. Use "idx" to populate with index of row instead of another oid.
}

type MIBTree struct {
	BaseOid string
	Tags    []MIBTag
	Metrics []MIBMetric
}

type ProcessDotNet struct {
	Name string
}

type HTTPUnit struct {
	Hiera string
	TOML  string
}

type Riak struct {
	URL string
}

type RabbitMQ struct {
	URL string
}

type Github struct {
	Repo  string
	Token string
}
