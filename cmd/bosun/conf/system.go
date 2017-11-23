package conf

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"bosun.org/cmd/bosun/expr"
	"bosun.org/graphite"
	"bosun.org/opentsdb"
	"github.com/BurntSushi/toml"
	"github.com/influxdata/influxdb/client/v2"
)

// SystemConf contains all the information that bosun needs to run. Outside of the conf package
// usage should be through conf.SystemConfProvider
type SystemConf struct {
	HTTPListen  string
	HTTPSListen string
	TLSCertFile string
	TLSKeyFile  string

	Hostname      string
	Ping          bool
	PingDuration  Duration // Duration from now to stop pinging hosts based on time since the host tag was touched
	TimeAndDate   []int    // timeanddate.com cities list
	SearchSince   Duration
	ShortURLKey   string
	InternetProxy string
	MinGroupSize  int

	UnknownThreshold int
	CheckFrequency   Duration // Time between alert checks: 5m
	DefaultRunEvery  int      // Default number of check intervals to run each alert: 1

	DBConf DBConf

	SMTPConf SMTPConf

	RuleVars map[string]string

	OpenTSDBConf OpenTSDBConf
	GraphiteConf GraphiteConf
	InfluxConf   InfluxConf
	ElasticConf  map[string]ElasticConf

	AnnotateConf AnnotateConf

	AuthConf *AuthConf

	MaxRenderedTemplateAge int // in days

	EnableSave      bool
	EnableReload    bool
	CommandHookPath string
	RuleFilePath    string
	md              toml.MetaData
}

// EnabledBackends stores which query backends supported by bosun are enabled
// via the system configuration. This is used so it can be passed to the rule parser
// and the parse errors can be thrown for query functions that are used when the backend
// is not enabled
type EnabledBackends struct {
	OpenTSDB bool
	Graphite bool
	Influx   bool
	Elastic  bool
	Logstash bool
	Annotate bool
}

// EnabledBackends returns and EnabledBackends struct which contains fields
// to state if a backend is enabled in the configuration or not
func (sc *SystemConf) EnabledBackends() EnabledBackends {
	b := EnabledBackends{}
	b.OpenTSDB = sc.OpenTSDBConf.Host != ""
	b.Graphite = sc.GraphiteConf.Host != ""
	b.Influx = sc.InfluxConf.URL != ""
	b.Elastic = len(sc.ElasticConf["default"].Hosts) != 0
	b.Annotate = len(sc.AnnotateConf.Hosts) != 0
	return b
}

// OpenTSDBConf contains OpenTSDB specific configuration information. The ResponseLimit
// will prevent Bosun from loading responses larger than its size in bytes. The version
// enables certain features of OpenTSDB querying
type OpenTSDBConf struct {
	ResponseLimit int64
	Host          string           // OpenTSDB relay and query destination: ny-devtsdb04:4242
	Version       opentsdb.Version // If set to 2.2 , enable passthrough of wildcards and filters, and add support for groupby
}

// GraphiteConf contains a string representing the host of a graphite server and
// a map of headers to be sent with each Graphite request
type GraphiteConf struct {
	Host    string
	Headers map[string]string
}

// AnnotateConf contains the elastic configuration to enable Annotations support
type AnnotateConf struct {
	Hosts         []string        // CSV of Elastic Hosts, currently the only backend in annotate
	SimpleClient  bool            // If true ES will connect over NewSimpleClient
	ClientOptions ESClientOptions // ES client options
	Index         string          // name of index / table
}

// ESClientOptions: elastic search client options
// reference https://github.com/olivere/elastic/blob/release-branch.v3/client.go#L107
type ESClientOptions struct {
	Enabled                   bool          // if true use client option else ignore
	BasicAuthUsername         string        // username for HTTP Basic Auth
	BasicAuthPassword         string        // password for HTTP Basic Auth
	Scheme                    string        // https (default http)
	SnifferEnabled            bool          // sniffer enabled or disabled
	SnifferTimeoutStartup     time.Duration // in seconds (default is 5 sec)
	SnifferTimeout            time.Duration // in seconds (default is 2 sec)
	SnifferInterval           time.Duration // in minutes (default is 15 min)
	HealthcheckEnabled        bool          // healthchecks enabled or disabled
	HealthcheckTimeoutStartup time.Duration // in seconds (default is 5 sec)
	HealthcheckTimeout        time.Duration // in seconds (default is 1 sec)
	HealthcheckInterval       time.Duration // in seconds (default is 60 sec)
	MaxRetries                int           // max. number of retries before giving up (default 10)
	GzipEnabled               bool          // enables or disables gzip compression (disabled by default)

}

// ElasticConf contains configuration for an elastic host that Bosun can query
type ElasticConf struct {
	Hosts         []string
	SimpleClient  bool
	ClientOptions ESClientOptions
}

// InfluxConf contains configuration for an influx host that Bosun can query
type InfluxConf struct {
	URL       string
	Username  string
	Password  string `json:"-"`
	UserAgent string
	Timeout   Duration
	UnsafeSSL bool
	Precision string
}

// DBConf stores the connection information for Bosun's internal storage
type DBConf struct {
	RedisHost     string
	RedisDb       int
	RedisPassword string

	LedisDir      string
	LedisBindAddr string
}

// SMTPConf contains information for the mail server for which bosun will
// send emails through
type SMTPConf struct {
	EmailFrom string
	Host      string
	Username  string
	Password  string `json:"-"`
}

//AuthConf is configuration for bosun's authentication
type AuthConf struct {
	AuthDisabled bool
	//Secret string to hash auth tokens. Needed to enable token auth.
	TokenSecret string
	//Secret sting used to encrypt cookie.
	CookieSecret string
	//LDAP configuration
	LDAP LDAPConf
}

type LDAPConf struct {
	// Domain name (used to make domain/username)
	Domain string
	//user base dn (LDAP Auth)
	UserBaseDn string
	// LDAP server
	LdapAddr string
	// allow insecure ldap connection?
	AllowInsecure bool
	// default permission level for anyone who can log in. Try "Reader".
	DefaultPermission string
	//List of group level permissions
	Groups []LDAPGroup
	//List of user specific permission levels
	Users map[string]string
	//Root search path for group lookups. Usually something like "DC=myorg,DC=com".
	//Only needed if using group permissions
	RootSearchPath string
}

//LDAPGroup is a Group level access specification for ldap
type LDAPGroup struct {
	// group search path string
	Path string
	// Access to grant members of group Ex: "Admin"
	Role string
}

// GetSystemConfProvider returns the SystemConfProvider interface
// and validates the logic of the configuration. If the configuration
// is not valid an error is returned
func (sc *SystemConf) GetSystemConfProvider() (SystemConfProvider, error) {
	var provider SystemConfProvider = sc
	if err := ValidateSystemConf(sc); err != nil {
		return provider, err
	}
	return provider, nil
}

const (
	defaultHTTPListen = ":8070"
)

// NewSystemConf retruns a system conf with default values set
func newSystemConf() *SystemConf {
	return &SystemConf{
		CheckFrequency:  Duration{Duration: time.Minute * 5},
		DefaultRunEvery: 1,
		HTTPListen:      defaultHTTPListen,
		DBConf: DBConf{
			LedisDir:      "ledis_data",
			LedisBindAddr: "127.0.0.1:9565",
		},
		MinGroupSize: 5,
		PingDuration: Duration{Duration: time.Hour * 24},
		OpenTSDBConf: OpenTSDBConf{
			ResponseLimit: 1 << 20, // 1MB
			Version:       opentsdb.Version2_1,
		},
		SearchSince:      Duration{time.Duration(opentsdb.Day) * 3},
		UnknownThreshold: 5,
	}
}

// LoadSystemConfigFile loads the system configuration in TOML format. It will
// error if there are values in the config that were not parsed
func LoadSystemConfigFile(fileName string) (*SystemConf, error) {
	return loadSystemConfig(fileName, true)
}

// LoadSystemConfig is like LoadSystemConfigFile but loads the config from a string
func LoadSystemConfig(conf string) (*SystemConf, error) {
	return loadSystemConfig(conf, false)
}

func loadSystemConfig(conf string, isFileName bool) (*SystemConf, error) {
	sc := newSystemConf()
	var decodeMeta toml.MetaData
	var err error
	if isFileName {
		decodeMeta, err = toml.DecodeFile(conf, &sc)
	} else {
		decodeMeta, err = toml.Decode(conf, &sc)
	}
	if err != nil {
		return sc, err
	}
	if len(decodeMeta.Undecoded()) > 0 {
		return sc, fmt.Errorf("undecoded fields in system configuration: %v", decodeMeta.Undecoded())
	}

	// iterate over each hosts
	for hostPrefix, value := range sc.ElasticConf {
		if value.SimpleClient && value.ClientOptions.Enabled {
			return sc, fmt.Errorf("Can't use both ES SimpleClient and ES ClientOptions please remove or disable one in ElasticConf.%s: %#v", hostPrefix, sc.ElasticConf)
		}
	}

	if sc.AnnotateConf.SimpleClient && sc.AnnotateConf.ClientOptions.Enabled {
		return sc, fmt.Errorf("Can't use both ES SimpleClient and ES ClientOptions please remove or disable one in AnnotateConf: %#v", sc.AnnotateConf)
	}

	sc.md = decodeMeta
	// clear default http listen if not explicitly specified
	if !decodeMeta.IsDefined("HTTPListen") && decodeMeta.IsDefined("HTTPSListen") {
		sc.HTTPListen = ""
	}
	return sc, nil
}

// GetHTTPListen returns the hostname:port that Bosun should listen on
func (sc *SystemConf) GetHTTPListen() string {
	return sc.HTTPListen
}

// GetHTTPSListen returns the hostname:port that Bosun should listen on with tls
func (sc *SystemConf) GetHTTPSListen() string {
	return sc.HTTPSListen
}

// GetTLSCertFile returns the path to the tls certificate to listen with (pem format). Must be specified with HTTPSListen.
func (sc *SystemConf) GetTLSCertFile() string {
	return sc.TLSCertFile
}

// GetTLSKeyFile returns the path to the tls key to listen with (pem format). Must be specified with HTTPSListen.
func (sc *SystemConf) GetTLSKeyFile() string {
	return sc.TLSKeyFile
}

// GetSMTPHost returns the SMTP mail server host that Bosun will use to relay through
func (sc *SystemConf) GetSMTPHost() string {
	return sc.SMTPConf.Host
}

// GetSMTPUsername returns the SMTP username that Bosun will use to connect to the mail server
func (sc *SystemConf) GetSMTPUsername() string {
	return sc.SMTPConf.Username
}

// GetSMTPPassword returns the SMTP password that Bosun will use to connect to the mail server
func (sc *SystemConf) GetSMTPPassword() string {
	return sc.SMTPConf.Password
}

// GetEmailFrom returns the email address that Bosun will use to send mail notifications from
func (sc *SystemConf) GetEmailFrom() string {
	return sc.SMTPConf.EmailFrom
}

// GetPing returns if Bosun's pinging is enabled. When Ping is enabled, bosun will ping all hosts
// that is has indexed and record metrics about those pings.
func (sc *SystemConf) GetPing() bool {
	return sc.Ping
}

// GetPingDuration returns the duration that discovered hosts (will be pinged until
// the host is not seen.
func (sc *SystemConf) GetPingDuration() time.Duration {
	return sc.PingDuration.Duration
}

// GetLedisDir returns the directory where Ledis should store its files
func (sc *SystemConf) GetLedisDir() string {
	return sc.DBConf.LedisDir
}

// GetLedisBindAddr returns the address that Ledis should listen on
func (sc *SystemConf) GetLedisBindAddr() string {
	return sc.DBConf.LedisBindAddr
}

// GetRedisHost returns the host to use for Redis. If this is set than Redis
// will be used instead of Ledis.
func (sc *SystemConf) GetRedisHost() string {
	return sc.DBConf.RedisHost
}

// GetRedisDb returns the redis database number to use
func (sc *SystemConf) GetRedisDb() int {
	return sc.DBConf.RedisDb
}

// GetRedisPassword returns the password that should be used to connect to redis
func (sc *SystemConf) GetRedisPassword() string {
	return sc.DBConf.RedisPassword
}

func (sc *SystemConf) GetAuthConf() *AuthConf {
	return sc.AuthConf
}

// GetRuleVars user defined variables that will be available to the rule configuration
// under "$sys.". This is so values with secrets can be defined in the system configuration
func (sc *SystemConf) GetRuleVars() map[string]string {
	return sc.RuleVars
}

// GetTimeAndDate returns the http://www.timeanddate.com/ that should be available to the UI
// so it can show links to translate UTC times to various timezones. This feature is only
// for creating UI Links as Bosun is expected to be running on a machine that is set to UTC
func (sc *SystemConf) GetTimeAndDate() []int {
	return sc.TimeAndDate
}

// GetSearchSince returns the duration that certain search requests should filter out results
// if they are older (have not been indexed) since the duration
func (sc *SystemConf) GetSearchSince() time.Duration {
	return sc.SearchSince.Duration
}

// GetCheckFrequency returns the default CheckFrequency that the schedule should run at. Checks by
// default will run at CheckFrequency * RunEvery
func (sc *SystemConf) GetCheckFrequency() time.Duration {
	return sc.CheckFrequency.Duration
}

// GetDefaultRunEvery returns the default multipler of how often an alert should run based on
// the CheckFrequency. Checks by default will run at CheckFrequency * RunEvery
func (sc *SystemConf) GetDefaultRunEvery() int {
	return sc.DefaultRunEvery
}

// GetUnknownThreshold returns the threshold in which multiple unknown alerts in a check iteration
// should be grouped into a single notification
func (sc *SystemConf) GetUnknownThreshold() int {
	return sc.UnknownThreshold
}

// GetMinGroupSize returns the minimum number of alerts needed to group the alerts
// on Bosun's dashboard
func (sc *SystemConf) GetMinGroupSize() int {
	return sc.MinGroupSize
}

// GetShortURLKey returns the API key that should be used to generate https://goo.gl/ shortlinks
// from Bosun's UI
func (sc *SystemConf) GetShortURLKey() string {
	return sc.ShortURLKey
}

// GetInternetProxy sets a proxy for outgoing network requests from Bosun. Currently it
// only impacts requests made for shortlinks to https://goo.gl/
func (sc *SystemConf) GetInternetProxy() string {
	return sc.InternetProxy
}

// MaxRenderedTemplateAge returns the maximum time in days to keep rendered templates
// after the incident end date.
func (sc *SystemConf) GetMaxRenderedTemplateAge() int {
	return sc.MaxRenderedTemplateAge
}

// SaveEnabled returns if saving via the UI and config editing API endpoints should be enabled
func (sc *SystemConf) SaveEnabled() bool {
	return sc.EnableSave
}

// ReloadEnabled returns if reloading of the rule config should be enabled. This will return
// true if save is enabled but reload is not enabled.
func (sc *SystemConf) ReloadEnabled() bool {
	return sc.EnableSave || sc.EnableReload
}

// GetCommandHookPath returns the path of a command that should be run on every save
func (sc *SystemConf) GetCommandHookPath() string {
	return sc.CommandHookPath
}

// GetRuleFilePath returns the path to the file containing contains rules
// rules include Alerts, Macros, Notifications, Templates, and Global Variables
func (sc *SystemConf) GetRuleFilePath() string {
	return sc.RuleFilePath
}

// SetTSDBHost sets the OpenTSDB host and used when Bosun is set to readonly mode
func (sc *SystemConf) SetTSDBHost(tsdbHost string) {
	sc.OpenTSDBConf.Host = tsdbHost
}

// GetTSDBHost returns the configured TSDBHost
func (sc *SystemConf) GetTSDBHost() string {
	return sc.OpenTSDBConf.Host
}

// GetAnnotateElasticHosts returns the Elastic hosts that should be used for annotations.
// Annotations are not enabled if this has no hosts
func (sc *SystemConf) GetAnnotateElasticHosts() expr.ElasticConfig {
	return parseESAnnoteConfig(sc)
}

// GetAnnotateIndex returns the name of the Elastic index that should be used for annotations
func (sc *SystemConf) GetAnnotateIndex() string {
	return sc.AnnotateConf.Index
}

// GetTSDBContext returns an OpenTSDB context limited to
// c.ResponseLimit. A nil context is returned if TSDBHost is not set.
func (sc *SystemConf) GetTSDBContext() opentsdb.Context {
	if sc.OpenTSDBConf.Host == "" {
		return nil
	}
	return opentsdb.NewLimitContext(sc.OpenTSDBConf.Host, sc.OpenTSDBConf.ResponseLimit, sc.OpenTSDBConf.Version)
}

// GetGraphiteContext returns a Graphite context which contains all the information needed
// to query Graphite. A nil context is returned if GraphiteHost is not set.
func (sc *SystemConf) GetGraphiteContext() graphite.Context {
	if sc.GraphiteConf.Host == "" {
		return nil
	}
	if len(sc.GraphiteConf.Headers) > 0 {
		headers := http.Header(make(map[string][]string))
		for k, v := range sc.GraphiteConf.Headers {
			headers.Add(k, v)
		}
		return graphite.HostHeader{
			Host:   sc.GraphiteConf.Host,
			Header: headers,
		}
	}
	return graphite.Host(sc.GraphiteConf.Host)
}

// GetInfluxContext returns a Influx context which contains all the information needed
// to query Influx.
func (sc *SystemConf) GetInfluxContext() client.HTTPConfig {
	c := client.HTTPConfig{}
	if sc.md.IsDefined("InfluxConf", "URL") {
		c.Addr = sc.InfluxConf.URL
	}
	if sc.md.IsDefined("InfluxConf", "Username") {
		c.Username = sc.InfluxConf.Username
	}
	if sc.md.IsDefined("InfluxConf", "Password") {
		c.Password = sc.InfluxConf.Password
	}
	if sc.md.IsDefined("InfluxConf", "UserAgent") {
		c.UserAgent = sc.InfluxConf.UserAgent
	}
	if sc.md.IsDefined("InfluxConf", "Timeout") {
		c.Timeout = sc.InfluxConf.Timeout.Duration
	}
	if sc.md.IsDefined("InfluxConf", "UnsafeSsl") {
		c.InsecureSkipVerify = sc.InfluxConf.UnsafeSSL
	}
	return c
}

// GetElasticContext returns an Elastic context which contains all the information
// needed to run Elastic queries.
func (sc *SystemConf) GetElasticContext() expr.ElasticHosts {
	return parseESConfig(sc)
}

// AnnotateEnabled returns if annotations have been enabled or not
func (sc *SystemConf) AnnotateEnabled() bool {
	return len(sc.AnnotateConf.Hosts) != 0
}

// MakeLink creates a HTML Link based on Bosun's configured Hostname
func (sc *SystemConf) MakeLink(path string, v *url.Values) string {
	u := url.URL{
		Scheme:   "http",
		Host:     sc.Hostname,
		Path:     path,
		RawQuery: v.Encode(),
	}
	return u.String()
}

// Duration is a time.Duration with a UnmarshalText method so
// durations can be decoded from TOML.
type Duration struct {
	time.Duration
}

// UnmarshalText is the method called by TOML when decoding a value
func (d *Duration) UnmarshalText(text []byte) error {
	var err error
	d.Duration, err = time.ParseDuration(string(text))
	return err
}

// URL is a *url.URL with a UnmarshalText method so
// a url can be decoded from TOML.
type URL struct {
	*url.URL
}

// UnmarshalText is the method called by TOML when decoding a value
func (u *URL) UnmarshalText(text []byte) error {
	var err error
	u.URL, err = url.Parse(string(bytes.Trim(text, `\"`)))
	return err
}
