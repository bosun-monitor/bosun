//Package database implements all persistent data access for bosun.
//Internally it runs ledisdb locally, but uses a redis client to access all data.
//Thus it should be able to migrate to a remote redis instance with minimal effort.
package database

import (
	"log"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/siddontang/ledisdb/config"
	"github.com/siddontang/ledisdb/server"
)

type DataAccess interface {
	PutMetricMetadata(metric string, field string, value string) error
	GetMetricMetadata(metric string) (*MetricMetadata, error)
}

type dataAccess struct {
	pool    *redis.Pool
	isRedis bool
}

func NewDataAccess(addr string, isRedis bool) DataAccess {
	return newDataAccess(addr, isRedis)
}

func newDataAccess(addr string, isRedis bool) *dataAccess {
	return &dataAccess{pool: newPool(addr, ""), isRedis: isRedis}
}

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

func newPool(server, password string) *redis.Pool {
	return &redis.Pool{
		MaxIdle:     3,
		IdleTimeout: 240 * time.Second,
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", server)
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
			_, err := c.Do("PING")
			return err
		},
	}
}
