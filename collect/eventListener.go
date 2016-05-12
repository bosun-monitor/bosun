package collect

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"bosun.org/opentsdb"
	"bosun.org/slog"
	"github.com/garyburd/redigo/redis"
)

func HandleCounterPut(server string, database int) http.HandlerFunc {

	pool := newRedisPool(server, database)
	return func(w http.ResponseWriter, r *http.Request) {
		gReader, err := gzip.NewReader(r.Body)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		decoder := json.NewDecoder(gReader)
		dps := []*opentsdb.DataPoint{}
		err = decoder.Decode(&dps)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		conn := pool.Get()
		defer conn.Close()
		for _, dp := range dps {
			mts := fmt.Sprintf("%s%s", dp.Metric, dp.Tags)
			var i int64
			switch v := dp.Value.(type) {
			case int:
				i = int64(v)
			case int64:
				i = v
			case float64:
				i = int64(v)
			default:
				http.Error(w, "Values must be integers.", 400)
				return
			}
			if _, err = conn.Do("HINCRBY", RedisCountersKey, mts, i); err != nil {
				slog.Errorf("Error incrementing counter %s by %d. %s", mts, i, err)
				http.Error(w, err.Error(), 500)
				return
			}
		}
	}
}

const RedisCountersKey = "scollectorCounters"

func newRedisPool(server string, database int) *redis.Pool {
	return &redis.Pool{
		MaxIdle:     10,
		MaxActive:   10,
		Wait:        true,
		IdleTimeout: 240 * time.Second,
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", server, redis.DialDatabase(database))
			if err != nil {
				return nil, err
			}
			return c, err
		},
	}
}
