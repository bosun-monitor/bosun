package database

import (
	"encoding/json"
	"log"
	"time"

	"bosun.org/models"
	"github.com/garyburd/redigo/redis"
)

/*

Silences : hash of Id - json of silence. Id is sha of fields

SilencesByEnd : zlist of end-time to id.

Easy to find active. Find all with end time in future, and filter to those with start time in the past.

*/

const (
	silenceHash = "Silences"
	silenceIdx  = "SilencesByEnd"
)

type SilenceDataAccess interface {
	GetActiveSilences() ([]*models.Silence, error)
	AddSilence(*models.Silence) error
	DeleteSilence(id string) error

	ListSilences(endingAfter int64) (map[string]*models.Silence, error)
}

func (d *dataAccess) Silence() SilenceDataAccess {
	return d
}

func (d *dataAccess) GetActiveSilences() ([]*models.Silence, error) {
	conn := d.Get()
	defer conn.Close()

	now := time.Now().UTC()
	vals, err := redis.Strings(conn.Do("ZRANGEBYSCORE", silenceIdx, now.Unix(), "+inf"))
	if err != nil {
		return nil, err
	}
	if len(vals) == 0 {
		return nil, nil
	}
	silences, err := getSilences(vals, conn)
	if err != nil {
		return nil, err
	}
	filtered := make([]*models.Silence, 0, len(silences))
	for _, s := range silences {
		if s.Start.After(now) {
			continue
		}
		filtered = append(filtered, s)
	}
	return filtered, nil
}

func getSilences(ids []string, conn redis.Conn) ([]*models.Silence, error) {
	args := make([]interface{}, len(ids)+1)
	args[0] = silenceHash
	for i := range ids {
		args[i+1] = ids[i]
	}
	jsons, err := redis.Strings(conn.Do("HMGET", args...))
	if err != nil {
		log.Fatal(err, args)
		return nil, err
	}
	silences := make([]*models.Silence, 0, len(jsons))
	for _, j := range jsons {
		s := &models.Silence{}
		if err := json.Unmarshal([]byte(j), s); err != nil {
			return nil, err
		}
		silences = append(silences, s)
	}
	return silences, nil
}

func (d *dataAccess) AddSilence(s *models.Silence) error {
	conn := d.Get()
	defer conn.Close()

	if _, err := conn.Do("ZADD", silenceIdx, s.End.UTC().Unix(), s.ID()); err != nil {
		return err
	}
	dat, err := json.Marshal(s)
	if err != nil {
		return err
	}
	_, err = conn.Do("HSET", silenceHash, s.ID(), dat)
	return err
}

func (d *dataAccess) DeleteSilence(id string) error {
	conn := d.Get()
	defer conn.Close()

	if _, err := conn.Do("ZREM", silenceIdx, id); err != nil {
		return err
	}
	if _, err := conn.Do("HDEL", silenceHash, id); err != nil {
		return err
	}
	return nil
}

func (d *dataAccess) ListSilences(endingAfter int64) (map[string]*models.Silence, error) {
	conn := d.Get()
	defer conn.Close()

	ids, err := redis.Strings(conn.Do("ZRANGEBYSCORE", silenceIdx, endingAfter, "+inf"))
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return map[string]*models.Silence{}, nil
	}
	silences, err := getSilences(ids, conn)
	if err != nil {
		return nil, err
	}
	m := make(map[string]*models.Silence, len(silences))
	for _, s := range silences {
		m[s.ID()] = s
	}
	return m, nil
}
