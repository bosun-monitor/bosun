package database

import (
	"crypto/md5"
	"encoding/base64"

	"bosun.org/_third_party/github.com/garyburd/redigo/redis"

	"bosun.org/collect"
	"bosun.org/opentsdb"
)

type ConfigDataAccess interface {
	SaveTempConfig(text string) (hash string, err error)
	GetTempConfig(hash string) (text string, err error)
}

func (d *dataAccess) Configs() ConfigDataAccess {
	return d
}

const configLifetime = 60 * 24 * 14 // 2 weeks

func (d *dataAccess) SaveTempConfig(text string) (string, error) {
	defer collect.StartTimer("redis", opentsdb.TagSet{"op": "SaveTempConfig"})()
	conn := d.GetConnection()
	defer conn.Close()

	sig := md5.Sum([]byte(text))
	b64 := base64.StdEncoding.EncodeToString(sig[0:8])
	_, err := conn.Do("SET", "tempConfig:"+b64, text, "EX", configLifetime)
	return b64, err
}

func (d *dataAccess) GetTempConfig(hash string) (string, error) {
	defer collect.StartTimer("redis", opentsdb.TagSet{"op": "GetTempConfig"})()
	conn := d.GetConnection()
	defer conn.Close()

	key := "tempConfig:" + hash
	dat, err := redis.String(conn.Do("GET", key))
	if err != nil {
		return "", err
	}
	_, err = conn.Do("EXPIRE", key, configLifetime)
	return dat, err
}
