//Package database implements all persistent data access for bosun.
//Internally it runs ledisdb locally, but uses a redis client to access all data.
//Thus it should be able to migrate to a remote redis instance with minimal effort.
package database

import (
	"bytes"
	"encoding/json"
	"log"
	"runtime"
	"strings"
	"time"

	"bosun.org/collect"
	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"bosun.org/slog"
	"github.com/garyburd/redigo/redis"
	"github.com/siddontang/ledisdb/config"
	"github.com/siddontang/ledisdb/server"

	"compress/gzip"
	"github.com/captncraig/easyauth/providers/token/redisStore"
)

var SchemaVersion = int64(1)

// Core data access interface for everything sched needs
type DataAccess interface {
	RedisConnector
	Metadata() MetadataDataAccess
	Configs() ConfigDataAccess
	Search() SearchDataAccess
	Errors() ErrorDataAccess
	State() StateDataAccess
	Silence() SilenceDataAccess
	Notifications() NotificationDataAccess
	Migrate() error
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
func NewDataAccess(addr string, isRedis bool, redisDb int, redisPass string) DataAccess {
	return newDataAccess(addr, isRedis, redisDb, redisPass)
}

func newDataAccess(addr string, isRedis bool, redisDb int, redisPass string) *dataAccess {
	return &dataAccess{
		pool:    newPool(addr, redisPass, redisDb, isRedis, 1000, true),
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

//RedisConnector is a simple interface so things can get a raw connection (mostly tests), but still discourage it.
// makes dataAccess interchangable with redis.Pool
type RedisConnector interface {
	Get() redis.Conn
}

//simple wrapper around a redis conn. Uses close to also stop and submit a simple timer for bosun stats on operations.
type connWrapper struct {
	redis.Conn
	closer func()
}

func (c *connWrapper) Close() error {
	err := c.Conn.Close()
	c.closer()
	return err
}

func (d *dataAccess) Get() redis.Conn {
	closer := collect.StartTimer("redis", opentsdb.TagSet{"op": myCallerName()})
	return &connWrapper{
		Conn:   d.pool.Get(),
		closer: closer,
	}
}

var _ redisStore.Connector = (*dataAccess)(nil) //just a compile time interface check

//gets name of function that called the currently executing function.
func myCallerName() string {
	fpcs := make([]uintptr, 1)
	runtime.Callers(3, fpcs)
	fun := runtime.FuncForPC(fpcs[0])
	nameSplit := strings.Split(fun.Name(), ".")
	return nameSplit[len(nameSplit)-1]
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

func (d *dataAccess) HCLEAR() string {
	if d.isRedis {
		return "DEL"
	}
	return "HCLEAR"
}

func (d *dataAccess) LMCLEAR(key string, value string) (string, []interface{}) {
	if d.isRedis {
		return "LREM", []interface{}{key, 0, value}
	}
	return "LMCLEAR", []interface{}{key, value}
}

func (d *dataAccess) HSCAN() string {
	if d.isRedis {
		return "HSCAN"
	}
	return "XHSCAN"
}

func (d *dataAccess) WriteJSON(conn redis.Conn, key string, data interface{}) error {
	dat, err := json.Marshal(data)
	if err != nil {
		return slog.Wrap(err)
	}
	if _, err = conn.Do("SET", key, dat); err != nil {
		return slog.Wrap(err)
	}
	return nil
}

func (d *dataAccess) WriteCompressedJSON(conn redis.Conn, key string, data interface{}) error {
	buf := &bytes.Buffer{}
	gz := gzip.NewWriter(buf)
	j := json.NewEncoder(gz)
	if err := j.Encode(data); err != nil {
		return slog.Wrap(err)
	}
	if err := gz.Close(); err != nil {
		return slog.Wrap(err)
	}
	if _, err := conn.Do("SET", key, buf.Bytes()); err != nil {
		return slog.Wrap(err)
	}
	return nil
}

func (d *dataAccess) ReadCompressedJSON(conn redis.Conn, key string, dst interface{}) error {
	dat, err := redis.Bytes(conn.Do("GET", key))
	if err != nil {
		return slog.Wrap(err)
	}
	gz, err := gzip.NewReader(bytes.NewReader(dat))
	if err != nil {
		return slog.Wrap(err)
	}
	j := json.NewDecoder(gz)
	return j.Decode(dst)
}

func (d *dataAccess) ReadJSON(conn redis.Conn, key string, dst interface{}) error {
	dat, err := redis.Bytes(conn.Do("GET", key))
	if err != nil {
		return slog.Wrap(err)
	}
	return json.Unmarshal(dat, dst)
}
