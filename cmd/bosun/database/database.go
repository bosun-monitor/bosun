//Package database implements all persistent data access for bosun.
//Internally it runs ledisdb locally, but uses a redis client to access all data.
//Thus it should be able to migrate to a remote redis instance with minimal effort.
package database

import (
	"log"
	"time"

	"bosun.org/_third_party/github.com/garyburd/redigo/redis"
	"bosun.org/_third_party/github.com/siddontang/ledisdb/config"
	"bosun.org/_third_party/github.com/siddontang/ledisdb/server"
	"bosun.org/collect"
	"bosun.org/metadata"
	"bosun.org/opentsdb"
)

// Core data access interface for everything sched needs
type DataAccess interface {
	Metadata() MetadataDataAccess
	Search() SearchDataAccess
	Errors() ErrorDataAccess
	Silence() SilenceDataAccess
	Incidents() IncidentDataAccess
}

type MetadataDataAccess interface {
	// Insert Metric Metadata. Field must be one of "desc", "rate", or "unit".
	PutMetricMetadata(metric string, field string, value string) error
	// Get Metric Metadata for given metric.
	GetMetricMetadata(metric string) (*MetricMetadata, error)

	PutTagMetadata(tags opentsdb.TagSet, name string, value string, updated time.Time) error
	GetTagMetadata(tags opentsdb.TagSet, name string) ([]*TagMetadata, error)
	DeleteTagMetadata(tags opentsdb.TagSet, name string) error
}

type SearchDataAccess interface {
	AddMetricForTag(tagK, tagV, metric string, time int64) error
	GetMetricsForTag(tagK, tagV string) (map[string]int64, error)

	AddTagKeyForMetric(metric, tagK string, time int64) error
	GetTagKeysForMetric(metric string) (map[string]int64, error)

	AddMetric(metric string, time int64) error
	GetAllMetrics() (map[string]int64, error)

	AddTagValue(metric, tagK, tagV string, time int64) error
	GetTagValues(metric, tagK string) (map[string]int64, error)

	AddMetricTagSet(metric, tagSet string, time int64) error
	GetMetricTagSets(metric string, tags opentsdb.TagSet) (map[string]int64, error)

	BackupLastInfos(map[string]map[string]*LastInfo) error
	LoadLastInfos() (map[string]map[string]*LastInfo, error)
}

type dataAccess struct {
	pool    *redis.Pool
	isRedis bool
}

// Create a new data access object pointed at the specified address. isRedis parameter used to distinguish true redis from ledis in-proc.
func NewDataAccess(addr string, isRedis bool) DataAccess {
	return newDataAccess(addr, isRedis)
}

func newDataAccess(addr string, isRedis bool) *dataAccess {
	return &dataAccess{
		pool:    newPool(addr, "", 0, isRedis, 1000, true),
		isRedis: isRedis,
	}
}

// Start in-process ledis server. Data will go in the specified directory and it will bind to the given port.
// Return value is a function you can call to stop the server.
func StartLedis(dataDir string, bind string) (stop func(), err error) {
	cfg := config.NewConfigDefault()
	cfg.DBName = "goleveldb"
	cfg.Addr = bind
	cfg.DataDir = dataDir
	app, err := server.NewApp(cfg)
	if err != nil {
		log.Fatal(err)
		return func() {}, err
	}
	go app.Run()
	return app.Close, nil
}

//interface so things can get a raw connection (mostly tests), but still discourage it.
type Connector interface {
	GetConnection() redis.Conn
}

func (d *dataAccess) GetConnection() redis.Conn {
	return d.pool.Get()
}

func newPool(server, password string, database int, isRedis bool, maxActive int, wait bool) *redis.Pool {
	return &redis.Pool{
		MaxIdle:     50,
		MaxActive:   maxActive,
		Wait:        wait,
		IdleTimeout: 240 * time.Second,
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", server, redis.DialDatabase(database))
			if err != nil {
				return nil, err
			}
			if password != "" {
				if _, err := c.Do("AUTH", password); err != nil {
					c.Close()
					return nil, err
				}
			}
			if isRedis {
				if _, err := c.Do("CLIENT", "SETNAME", "bosun"); err != nil {
					c.Close()
					return nil, err
				}
			}
			return c, err
		},
	}
}

func init() {
	collect.AggregateMeta("bosun.redis", metadata.MilliSecond, "time in milliseconds per redis call.")
}

// Ledis can't do DEL in a blanket way like redis can. It has a unique command per type.
// These helpers allow easy switching.
func (d *dataAccess) LCLEAR() string {
	if d.isRedis {
		return "DEL"
	}
	return "LCLEAR"
}

func (d *dataAccess) SCLEAR() string {
	if d.isRedis {
		return "DEL"
	}
	return "SCLEAR"
}

func (d *dataAccess) LMCLEAR(key string, value string) (string, []interface{}) {
	if d.isRedis {
		return "LREM", []interface{}{key, 0, value}
	}
	return "LMCLEAR", []interface{}{key, value}
}
