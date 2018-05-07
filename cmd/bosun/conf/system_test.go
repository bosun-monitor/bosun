package conf

import (
	"testing"
	"time"

	"bosun.org/opentsdb"

	"github.com/stretchr/testify/assert"
)

func TestSystemToml(t *testing.T) {
	sc, err := LoadSystemConfigFile("test.toml")
	if err != nil {
		t.Errorf("failed to load/parse config file: %v", err)
		return
	}
	assert.Equal(t, sc.Hostname, "bosun.example.com", "Hostname not equal")
	assert.Equal(t, sc.Scheme, "https", "Scheme does not match")
	assert.Equal(t, sc.DefaultRunEvery, 5)
	assert.Equal(t, sc.CheckFrequency, Duration{time.Minute})
	assert.Equal(t, sc.Ping, true)
	assert.Equal(t, sc.MinGroupSize, 5)
	assert.Equal(t, sc.UnknownThreshold, 5)
	assert.Equal(t, sc.SearchSince, Duration{Duration: time.Hour * 72})
	assert.Equal(t, sc.PingDuration, Duration{Duration: time.Hour * 24}, "PingDuration does not match (should be set by default)")
	assert.Equal(t, sc.HTTPListen, ":8080", "HTTPListen does not match")
	assert.Equal(t, sc.TimeAndDate, []int{202, 75, 179, 136}, "TimeAndDate does not match")
	assert.Equal(t, sc.ShortURLKey, "aKey")
	assert.Equal(t, sc.EnableSave, false)
	assert.Equal(t, sc.CommandHookPath, "/Users/kbrandt/src/hook/hook")
	assert.Equal(t, sc.RuleFilePath, "dev.sample.conf")
	assert.Equal(t, sc.OpenTSDBConf, OpenTSDBConf{
		Host:          "ny-tsdb01:4242",
		ResponseLimit: 25000000,
		Version:       opentsdb.Version2_2,
	})
	assert.Equal(t, sc.GraphiteConf, GraphiteConf{
		Host:    "localhost:80",
		Headers: map[string]string{"X-Meow": "Mix"},
	})
	assert.Equal(t, sc.ElasticConf, map[string]ElasticConf{
		"default": {
			Hosts: []string{"http://ny-lselastic01.example.com:9200", "http://ny-lselastic02.example.com:9200"},
		},
	})
	assert.Equal(t, sc.AnnotateConf, AnnotateConf{
		Hosts: []string{"http://ny-lselastic01.example.com:9200", "http://ny-lselastic02.example.com:9200"},
	})
	assert.Equal(t, sc.DBConf, DBConf{
		RedisHost:          "localhost:6389", // From Config
		RedisClientSetName: true,
		LedisDir:           "ledis_data",     // Default
		LedisBindAddr:      "127.0.0.1:9565", // Default

	}, "DBConf does not match")
	assert.Equal(t, sc.SMTPConf, SMTPConf{
		EmailFrom: "bosun@example.com",
		Host:      "mail.example.com",
	}, "SMTPConf does not match")
	assert.Equal(t, sc.InfluxConf, InfluxConf{
		URL:       "https://myInfluxServer:1234",
		Timeout:   Duration{time.Minute * 5},
		UnsafeSSL: true,
	})
}
