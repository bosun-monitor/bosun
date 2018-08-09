package conf

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"bosun.org/slog"

	"bosun.org/cmd/bosun/expr"
	"bosun.org/graphite"
	"bosun.org/opentsdb"
	"github.com/Azure/azure-sdk-for-go/services/preview/monitor/mgmt/2018-03-01/insights"
	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2018-02-01/resources"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure/auth"
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
	Scheme        string // default http
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

	ExampleExpression string

	OpenTSDBConf     OpenTSDBConf
	GraphiteConf     GraphiteConf
	InfluxConf       InfluxConf
	ElasticConf      map[string]ElasticConf
	AzureMonitorConf map[string]AzureMonitorConf

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
	OpenTSDB     bool
	Graphite     bool
	Influx       bool
	Elastic      bool
	Logstash     bool
	Annotate     bool
	AzureMonitor bool
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
	b.AzureMonitor = len(sc.AzureMonitorConf) != 0
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

// AzureConf contains configuration for an Azure metrics
type AzureMonitorConf struct {
	SubscriptionId string
	TenantId       string
	ClientId       string
	ClientSecret   string
	Concurrency    int
	DebugRequest   bool
	DebugResponse  bool
}

// Valid returns if the configuration for the AzureMonitor has
// required fields with appropriate values
func (ac AzureMonitorConf) Valid() error {
	present := make(map[string]bool)
	missing := []string{}
	errors := []string{}
	present["SubscriptionId"] = ac.SubscriptionId != ""
	present["TenantId"] = ac.TenantId != ""
	present["ClientId"] = ac.ClientId != ""
	present["ClientSecret"] = ac.ClientSecret != ""
	for k, v := range present {
		if !v {
			missing = append(missing, k)
		}
	}
	if len(missing) != 0 {
		errors = append(errors, fmt.Sprintf("missing required fields: %v", strings.Join(missing, ", ")))
	} else {
		ccc := auth.NewClientCredentialsConfig(ac.ClientId, ac.ClientSecret, ac.TenantId)
		_, err := ccc.Authorizer() // We don't use the value here, only checking for error
		if err != nil {
			errors = append(errors, fmt.Sprintf("problem creating valid authorization: %v", err.Error()))
		}
	}
	if ac.Concurrency < 0 {
		errors = append(errors, fmt.Sprintf("concurrency is %v and must be 0 or greater", ac.Concurrency))
	}
	if len(errors) != 0 {
		return fmt.Errorf("%v", strings.Join(errors, " and "))
	}
	return nil
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
		Scheme:          "http",
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

	// Check Azure Monitor Configurations
	for prefix, conf := range sc.AzureMonitorConf {
		if err := conf.Valid(); err != nil {
			return sc, fmt.Errorf(`error in configuration for Azure client "%v": %v`, prefix, err)
		}
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

// GetExampleExpression returns the default expression for "Expression" tab.
func (sc *SystemConf) GetExampleExpression() string {
	return sc.ExampleExpression
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

// GetAzureMonitorContext returns a Azure Monitor API context which
// contains the information needs to query the azure API
func (sc *SystemConf) GetAzureMonitorContext() expr.AzureMonitorClients {
	allClients := make(expr.AzureMonitorClients)
	for prefix, conf := range sc.AzureMonitorConf {
		clients := expr.AzureMonitorClientCollection{}
		if conf.Concurrency == 0 {
			clients.Concurrency = 10
		} else {
			clients.Concurrency = conf.Concurrency
		}
		clients.MetricsClient = insights.NewMetricsClient(conf.SubscriptionId)
		clients.MetricDefinitionsClient = insights.NewMetricDefinitionsClient(conf.SubscriptionId)
		clients.ResourcesClient = resources.NewClient(conf.SubscriptionId)
		if conf.DebugRequest {
			clients.ResourcesClient.RequestInspector, clients.MetricsClient.RequestInspector, clients.MetricDefinitionsClient.RequestInspector = azureLogRequest(), azureLogRequest(), azureLogRequest()
		}
		if conf.DebugResponse {
			clients.ResourcesClient.ResponseInspector, clients.MetricsClient.ResponseInspector, clients.MetricDefinitionsClient.ResponseInspector = azureLogResponse(), azureLogResponse(), azureLogResponse()
		}
		ccc := auth.NewClientCredentialsConfig(conf.ClientId, conf.ClientSecret, conf.TenantId)
		at, err := ccc.Authorizer()
		if err != nil {
			// Should not hit this since we check for authorizer errors in Validation
			// This is checked before because this method is not called until the an expression is called
			slog.Error("unexpected Azure Authorizer error: ", err)
		}
		clients.MetricsClient.Authorizer, clients.MetricDefinitionsClient.Authorizer, clients.ResourcesClient.Authorizer = at, at, at
		allClients[prefix] = clients
	}
	return allClients
}

// azureLogRequest outputs HTTP requests to Azure to the logs
func azureLogRequest() autorest.PrepareDecorator {
	return func(p autorest.Preparer) autorest.Preparer {
		return autorest.PreparerFunc(func(r *http.Request) (*http.Request, error) {
			r, err := p.Prepare(r)
			if err != nil {
				slog.Warningf("failure to dump azure request: %v", err)
			}
			dump, err := httputil.DumpRequestOut(r, true)
			if err != nil {
				slog.Warningf("failure to dump azure request: %v", err)
			}
			slog.Info(string(dump))
			return r, err
		})
	}
}

// azureLogRequest outputs HTTP responses from requests to Azure to the logs
func azureLogResponse() autorest.RespondDecorator {
	return func(p autorest.Responder) autorest.Responder {
		return autorest.ResponderFunc(func(r *http.Response) error {
			err := p.Respond(r)
			if err != nil {
				slog.Warningf("failure to dump azure response: %v", err)
			}
			dump, err := httputil.DumpResponse(r, true)
			if err != nil {
				slog.Warningf("failure to dump azure response: %v", err)
			}
			slog.Info(string(dump))
			return err
		})
	}
}

// AnnotateEnabled returns if annotations have been enabled or not
func (sc *SystemConf) AnnotateEnabled() bool {
	return len(sc.AnnotateConf.Hosts) != 0
}

// MakeLink creates a HTML Link based on Bosun's configured Hostname
func (sc *SystemConf) MakeLink(path string, v *url.Values) string {
	u := url.URL{
		Scheme: sc.Scheme,
		Host:   sc.Hostname,
		Path:   path,
	}
	if v != nil {
		u.RawQuery = v.Encode()
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
