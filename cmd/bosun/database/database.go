//Package database implements all persistent data access for bosun.
//Internally it runs ledisdb locally, but uses a redis client to access all data.
//Thus it should be able to migrate to a remote redis instance with minimal effort.
package database

import (
	"log"
	"time"

	"bosun.org/collect"
	"bosun.org/opentsdb"
	"github.com/garyburd/redigo/redis"
	"github.com/siddontang/ledisdb/config"
	"github.com/siddontang/ledisdb/server"
)

// Core data access interface for everything sched needs
type DataAccess interface {
	// Insert Metric Metadata. Field must be one of "desc", "rate", or "unit".
	PutMetricMetadata(metric string, field string, value string) error
	// Get Metric Metadata for given metric.
	GetMetricMetadata(metric string) (*MetricMetadata, error)

	PutTagMetadata(tags opentsdb.TagSet, name string, value string, updated time.Time) error
	GetTagMetadata(tags opentsdb.TagSet, name string) ([]*TagMetadata, error)
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
	return &dataAccess{pool: newPool(addr, "", 0), isRedis: isRedis}
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

func (d *dataAccess) getConnection() redis.Conn {
	return d.pool.Get()
}

func newPool(server, password string, database int) *redis.Pool {
	return &redis.Pool{
		MaxIdle:     3,
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
			return c, err
		},
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			defer collect.StartTimer("redis", opentsdb.TagSet{"op": "Ping"})()
			_, err := c.Do("PING")
			return err
		},
	}
}
