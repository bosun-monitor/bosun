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
	// MaxQueueLen is the number of metrics keept internally.
	MaxQueueLen int
	// MaxMem is the maximum number of megabytes that can be allocated
	// before scollector panics (shuts down). Default of 500 MB. This
	// is a saftey mechanism to protect the host from the monitoring
	// agent
	MaxMem uint64
	// Filter filters collectors matching these terms.
	Filter []string
	// PProf is an IP:Port binding to be used for debugging with pprof and expvar package.
	// When enabled data is exposed via http://host:port/debug/pprof or /debug/vars
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

	// UseNtlm specifies if HTTP requests should authenticate with NTLM.
	UseNtlm bool

	// AuthToken is an optional string that sets the X-Access-Token HTTP header
	// which is used to authenticate against Bosun
	AuthToken string

	// UserAgentMessage is an optional message that is appended to the User Agent
	UserAgentMessage string

	// SNMPTimeout is the number of seconds to wait for SNMP responses (default 30)
	SNMPTimeout int

	// UseSWbemServicesClient specifies if the wmi package should use SWbemServices.
	UseSWbemServicesClient bool

	// MetricPrefix prepended to all metrics path
	MetricPrefix string

	HAProxy        []HAProxy
	SNMP           []SNMP
	MIBS           map[string]MIB
	ICMP           []ICMP
	Vsphere        []Vsphere
	AWS            []AWS
	AzureEA        []AzureEA
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
	GoogleWebmaster     []GoogleWebmaster
	Cadvisor            []Cadvisor
	RedisCounters       []RedisCounters
	ExtraHop            []ExtraHop
	LocalListener       string
	TagOverride         []TagOverride
	HadoopHost          string
	HbaseRegions        bool
	Oracles             []Oracle
	Fastly              []Fastly
}

type HAProxy struct {
	User      string
	Password  string
	Instances []HAProxyInstance
}

type HAProxyInstance struct {
	User     string
	Password string
	Tier     string
	URL      string
}

type Nexpose struct {
	Username string
	Password string
	Host     string
	Insecure bool
}

type GoogleAnalytics struct {
	ClientID  string
	Secret    string
	Token     string
	JSONToken string
	Sites     []GoogleAnalyticsSite
}

type GoogleWebmaster struct {
	ClientID  string
	Secret    string
	Token     string
	JSONToken string
}

type Fastly struct {
	Key            string
	StatusBaseAddr string
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
	AccessKey                string
	SecretKey                string
	Region                   string
	BillingProductCodesRegex string
	BillingBucketName        string
	BillingBucketPath        string
	BillingPurgeDays         int
}

type AzureEA struct {
	EANumber          uint32
	APIKey            string
	LogBillingDetails bool
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
	Freq  string
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
	URL         string
	PerCpuUsage bool
	IsRemote    bool
}

type RedisCounters struct {
	Server   string
	Database int
}

type ExtraHop struct {
	Host                     string
	APIKey                   string
	FilterBy                 string
	FilterPercent            int
	AdditionalMetrics        []string
	CertificateSubjectMatch  string
	CertificateActivityGroup int
}

type TagOverride struct {
	CollectorExpr string
	MatchedTags   map[string]string
	Tags          map[string]string
}

type Oracle struct {
	ClusterName string
	Instances   []OracleInstance
}

type OracleInstance struct {
	ConnectionString string
	Role             string
}
