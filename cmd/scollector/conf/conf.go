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
	// BatchSize is the number of metrics that will be sent in each batch.
	BatchSize int
	// Filter filters collectors matching these terms.
	Filter []string
	// PProf is an IP:Port binding to be used for debugging with pprof package.
	// Examples: localhost:6060 for loopback or :6060 for all IP addresses.
	PProf string
	// MetricFilters takes regular expressions and includes only indicies that
	// match those filters from being monitored
	MetricFilters []string

	// KeepalivedCommunity, if not empty, enables the Keepalived collector with
	// the specified community.
	KeepalivedCommunity string

	//Override default network interface expression
	IfaceExpr string

	HAProxy        []HAProxy
	SNMP           []SNMP
	MIBS           map[string]MIB
	ICMP           []ICMP
	Vsphere        []Vsphere
	AWS            []AWS
	Process        []ProcessParams
	SystemdService []ServiceParams
	ProcessDotNet  []ProcessDotNet
	HTTPUnit       []HTTPUnit
	Riak           []Riak
	Github         []Github
	// ElasticIndexFilters takes regular expressions and excludes indicies that
	// match those filters from being monitored for metrics in the elastic.indices
	// namespace
	ElasticIndexFilters []string
	RabbitMQ            []RabbitMQ
	Nexpose             []Nexpose
	GoogleAnalytics     []GoogleAnalytics
	Cadvisor            []Cadvisor
	RedisCounters       []RedisCounters
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

type Nexpose struct {
	Username string
	Password string
	Host     string
	Insecure bool
}

type GoogleAnalytics struct {
	ClientID string
	Secret   string
	Token    string
	Sites    []GoogleAnalyticsSite
}

type GoogleAnalyticsSite struct {
	Name     string
	Profile  string
	Offset   int
	Detailed bool
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
	Metrics []MIBMetric // single key metrics
	Trees   []MIBTree   // tagged array metrics
}

type MIBMetric struct {
	Metric      string
	Oid         string
	Unit        string // metadata unit
	RateType    string // defaults to gauge
	Description string
	FallbackOid string // Oid to try if main one doesn't work. Used in cisco where different models use different oids
	Tags        string // static tags to populate for this metric. "direction=in"
	Scale       float64
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
	TOML  string
	Hiera string
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

type Cadvisor struct {
	URL string
}

type RedisCounters struct {
	Server   string
	Database int
}
