package conf

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"bosun.org/cmd/bosun/expr"
	"bosun.org/graphite"
	"bosun.org/opentsdb"
	"github.com/BurntSushi/toml"
	"github.com/influxdata/influxdb/client"
)

type SystemConf struct {
	Name          string
	HTTPListen    string
	RelayListen   string
	Hostname      string
	Ping          bool
	PingDuration  duration // Duration from now to stop pinging hosts based on time since the host tag was touched
	TimeAndDate   []int    // timeanddate.com cities list
	SearchSince   opentsdb.Duration
	ShortURLKey   string
	InternetProxy string
	MinGroupSize  int

	UnknownThreshold int
	CheckFrequency   duration // Time between alert checks: 5m
	DefaultRunEvery  int      // Default number of check intervals to run each alert: 1

	DBConf DBConf

	SMTPConf SMTPConf

	OpenTSDBConf OpenTSDBConf
	GraphiteConf GraphiteConf
	InfluxConf   InfluxConf
	ElasticConf  ElasticConf
	LogStashConf LogStashConf

	AnnotateConf AnnotateConf
}

type EnabledBackends struct {
	OpenTSDB bool
	Graphite bool
	Influx   bool
	Elastic  bool
	Logstash bool
}

func (sc *SystemConf) EnabledBackends() EnabledBackends {
	b := EnabledBackends{}
	b.OpenTSDB = sc.OpenTSDBConf.Host != ""
	b.Graphite = sc.GraphiteConf.Host != ""
	b.Influx = sc.InfluxConf.URL.Host != ""
	b.Logstash = len(sc.LogStashConf.Hosts) != 0
	b.Elastic = len(sc.ElasticConf.Hosts) != 0
	return b
}

type OpenTSDBConf struct {
	ResponseLimit int64
	Host          string            // OpenTSDB relay and query destination: ny-devtsdb04:4242
	Version       opentsdb.Version // If set to 2.2 , enable passthrough of wildcards and filters, and add support for groupby
}

type GraphiteConf struct {
	Host    string
	Headers []string
}

type AnnotateConf struct {
	Hosts []string // CSV of Elastic Hosts, currently the only backend in annotate
	Index string   // name of index / table
}

type LogStashConf struct {
	Hosts expr.LogstashElasticHosts
}

type ElasticConf struct {
	Hosts expr.ElasticHosts
}

type InfluxConf struct {
	client.Config
}

type DBConf struct {
	RedisHost     string
	RedisDb       int
	RedisPassword string

	LedisDir      string
	LedisBindAddr string
}

type SMTPConf struct {
	EmailFrom string
	Host      string
	Username  string
	Password  string `json:"-"`
}

func LoadSystemConfigFile(fileName string) (*SystemConf, error) {
	sc := &SystemConf{
		CheckFrequency:  duration{Duration: time.Minute * 5},
		DefaultRunEvery: 1,
		HTTPListen:      ":8070",
		DBConf: DBConf{
			LedisDir:      "ledis_data",
			LedisBindAddr: "127.0.0.1:9565",
		},
		MinGroupSize: 5,
		PingDuration: duration{Duration: time.Hour * 24},
		OpenTSDBConf: OpenTSDBConf{
			ResponseLimit: 1 << 20, // 1MB
			Version:       opentsdb.Version2_1,
		},
		SearchSince:      opentsdb.Day * 3,
		UnknownThreshold: 5,
	}
	fileContents, err := ioutil.ReadFile(fileName)
	if err != nil {
		return sc, fmt.Errorf("failed to load system config file: %v", err)
	}
	if _, err := toml.Decode(string(fileContents), &sc); err != nil {
		return sc, err
	}
	return sc, nil
}

type duration struct {
	time.Duration
}

func (d *duration) UnmarshalText(text []byte) error {
	var err error
	d.Duration, err = time.ParseDuration(string(text))
	return err
}

func (sc *SystemConf) GetHTTPListen() string {
	return sc.HTTPListen
}

func (sc *SystemConf) GetRelayListen() string {
	return sc.RelayListen
}

func (sc *SystemConf) GetSMTPHost() string {
	return sc.SMTPConf.Host
}

func (sc *SystemConf) GetSMTPUsername() string {
	return sc.SMTPConf.Username
}

func (sc *SystemConf) GetSMTPPassword() string {
	return sc.SMTPConf.Password
}

func (sc *SystemConf) GetEmailFrom() string {
	return sc.SMTPConf.EmailFrom
}

func (sc *SystemConf) GetPing() bool {
	return sc.Ping
}

func (sc *SystemConf) GetPingDuration() time.Duration {
	return sc.PingDuration.Duration
}

func (sc *SystemConf) GetLedisDir() string {
	return sc.DBConf.LedisDir
}

func (sc *SystemConf) GetLedisBindAddr() string {
	return sc.DBConf.LedisBindAddr
}

func (sc *SystemConf) GetRedisHost() string {
	return sc.DBConf.RedisHost
}

func (sc *SystemConf) GetRedisDb() int {
	return sc.DBConf.RedisDb
}

func (sc *SystemConf) GetRedisPassword() string {
	return sc.DBConf.RedisPassword
}

func (sc *SystemConf) GetTimeAndDate() []int {
	return sc.TimeAndDate
}

func (sc *SystemConf) GetResponseLimit() int64 {
	return sc.OpenTSDBConf.ResponseLimit
}

func (sc *SystemConf) GetSearchSince() opentsdb.Duration {
	return sc.SearchSince
}

func (sc *SystemConf) GetCheckFrequency() time.Duration {
	return sc.CheckFrequency.Duration
}

func (sc *SystemConf) GetDefaultRunEvery() int {
    return sc.DefaultRunEvery
}

func (sc *SystemConf) GetUnknownThreshold() int {
	return sc.UnknownThreshold
}

func (sc *SystemConf) GetMinGroupSize() int {
	return sc.MinGroupSize
}

func (sc *SystemConf) GetShortURLKey() string {
	return sc.ShortURLKey
}

func (sc *SystemConf) GetInternetProxy() string {
	return sc.InternetProxy
}

func (sc *SystemConf) SetTSDBHost(tsdbHost string) {
	sc.OpenTSDBConf.Host = tsdbHost
}

func (sc *SystemConf) GetTSDBHost() string {
	return sc.OpenTSDBConf.Host
}

func (sc *SystemConf) GetTSDBVersion() *opentsdb.Version {
	return &sc.OpenTSDBConf.Version
}

func (sc *SystemConf) GetGraphiteHost() string {
	return sc.GraphiteConf.Host
}

func (sc *SystemConf) GetGraphiteHeaders() []string {
	return sc.GraphiteConf.Headers
}

func (sc *SystemConf) GetLogstashElasticHosts() expr.LogstashElasticHosts {
	return sc.LogStashConf.Hosts
}

func (sc *SystemConf) GetAnnotateElasticHosts() expr.ElasticHosts {
	return sc.AnnotateConf.Hosts
}

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

// GraphiteContext returns a Graphite context. A nil context is returned if
// GraphiteHost is not set.
func (sc *SystemConf) GetGraphiteContext() graphite.Context {
	if sc.GraphiteConf.Host == "" {
		return nil
	}
	if len(sc.GraphiteConf.Headers) > 0 {
		headers := http.Header(make(map[string][]string))
		for _, s := range sc.GraphiteConf.Headers {
			kv := strings.Split(s, ":")
			headers.Add(kv[0], kv[1])
		}
		return graphite.HostHeader{
			Host:   sc.GraphiteConf.Host,
			Header: headers,
		}
	}
	return graphite.Host(sc.GraphiteConf.Host)
}

func (sc *SystemConf) GetInfluxContext() client.Config {
	return sc.InfluxConf.Config
}

func (sc *SystemConf) GetLogstashContext() expr.LogstashElasticHosts {
	return sc.LogStashConf.Hosts
}

func (sc *SystemConf) GetElasticContext() expr.ElasticHosts {
	return sc.ElasticConf.Hosts
}

func (sc *SystemConf) AnnotateEnabled() bool {
	return len(sc.AnnotateConf.Hosts) != 0
}

func (sc *SystemConf) MakeLink(path string, v *url.Values) string {
	u := url.URL{
		Scheme:   "http",
		Host:     sc.Hostname,
		Path:     path,
		RawQuery: v.Encode(),
	}
	return u.String()
}

// TODO Validation
// defaultRunEvery > 0
// The following to hostname?
	// if c.Hostname == "" {
	// 	c.Hostname = c.HTTPListen
	// 	if strings.HasPrefix(c.Hostname, ":") {
	// 		h, err := os.Hostname()
	// 		if err != nil {
	// 			c.at(nil)
	// 			c.error(err)
	// 		}
	// 		c.Hostname = h + c.Hostname
	// 	}
	// }

    // SMTP Validation:
    // if c.SMTPHost == "" || c.EmailFrom == "" {
	// 			c.errorf("email notifications require both smtpHost and emailFrom to be set")
	// 		}

