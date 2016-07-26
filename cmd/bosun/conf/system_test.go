package conf

import (
	"testing"
	"time"

	"bosun.org/cmd/bosun/expr"
	"bosun.org/opentsdb"

	"github.com/stretchr/testify/assert"
	"net/url"
)

func TestSystemToml(t *testing.T) {
	conf := `
Hostname = "bosun.example.com"
HTTPListen = ":8080"
TimeAndDate = [ 202, 75, 179, 136 ]
ShortURLKey = "aKey"
CommandHookPath = "/Users/kbrandt/src/hook/hook"
RuleFilePath = "/Users/kbrandt/src/testProdRepo/prod.conf"

[OpenTSDBConf]
    Host = "ny-tsdb01:4242"
    Version = 2.2
    ResponseLimit = 25000000

#Test comment

[ElasticConf]
    Hosts = ["http://ny-lselastic01.example.com:9200", "http://ny-lselastic02.example.com:9200"]

[DBConf]
    RedisHost = "localhost:6389"

[SMTPConf]
    EmailFrom = "bosun@example.com"
    Host = "mail.example.com"

[InfluxConf]
    URL = "https://myInfluxServer:1234"
    Timeout = "5m"
    UnsafeSSL = true

`
	sc, err := LoadSystemConfig(conf)
	if err != nil {
		t.Errorf("failed to parse config file: %v", err)
		return
	}
	_ = sc
	assert.Equal(t, sc.Hostname, "bosun.example.com", "Hostname not equal")
	assert.Equal(t, sc.PingDuration, Duration{Duration: time.Hour * 24}, "PingDuration does not match (should be set by default)")
	assert.Equal(t, sc.HTTPListen, ":8080", "HTTPListen does not match")
	assert.Equal(t, sc.TimeAndDate, []int{202, 75, 179, 136}, "TimeAndDate does not match")
	assert.Equal(t, sc.ShortURLKey, "aKey")
	assert.Equal(t, sc.CommandHookPath, "/Users/kbrandt/src/hook/hook")
	assert.Equal(t, sc.RuleFilePath, "/Users/kbrandt/src/testProdRepo/prod.conf")
	assert.Equal(t, sc.OpenTSDBConf, OpenTSDBConf{
		Host:          "ny-tsdb01:4242",
		ResponseLimit: 25000000,
		Version:       opentsdb.Version2_2,
	})
	assert.Equal(t, sc.ElasticConf, ElasticConf{
		Hosts: expr.ElasticHosts{"http://ny-lselastic01.example.com:9200", "http://ny-lselastic02.example.com:9200"},
	})
	assert.Equal(t, sc.DBConf, DBConf{
		RedisHost:     "localhost:6389", // From Config
		LedisDir:      "ledis_data",     // Default
		LedisBindAddr: "127.0.0.1:9565", // Default

	}, "DBConf does not match")
	assert.Equal(t, sc.SMTPConf, SMTPConf{
		EmailFrom: "bosun@example.com",
		Host:      "mail.example.com",
	}, "SMTPConf does not match")
	assert.Equal(t, sc.InfluxConf, InfluxConf{
		URL:       URL{&url.URL{Scheme: "https", Host: "myInfluxServer:1234"}},
		Timeout:   Duration{time.Minute * 5},
		UnsafeSSL: true,
	})
}
