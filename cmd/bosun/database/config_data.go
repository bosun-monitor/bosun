package database

import (
	"crypto/md5"
	"encoding/base64"
	"fmt"

	"github.com/garyburd/redigo/redis"

	"bosun.org/slog"
)

type ConfigDataAccess interface {
	SaveTempConfig(text string) (hash string, err error)
	GetTempConfig(hash string) (text string, err error)

	ShortenLink(fullURL string) (id int, err error)
	GetShortLink(id int) (fullURL string, err error)
}

func (d *dataAccess) Configs() ConfigDataAccess {
	return d
}

const configLifetime = 60 * 24 * 14 // 2 weeks

func (d *dataAccess) SaveTempConfig(text string) (string, error) {
	conn := d.Get()
	defer conn.Close()

	sig := md5.Sum([]byte(text))
	b64 := base64.StdEncoding.EncodeToString(sig[0:8])
	if d.isRedis {
		_, err := conn.Do("SET", "tempConfig:"+b64, text, "EX", configLifetime)
		return b64, slog.Wrap(err)
	}
	_, err := conn.Do("SETEX", "tempConfig:"+b64, configLifetime, text)
	return b64, slog.Wrap(err)
}

func (d *dataAccess) GetTempConfig(hash string) (string, error) {
	conn := d.Get()
	defer conn.Close()

	key := "tempConfig:" + hash
	dat, err := redis.String(conn.Do("GET", key))
	if err != nil {
		return "", slog.Wrap(err)
	}
	_, err = conn.Do("EXPIRE", key, configLifetime)
	return dat, slog.Wrap(err)
}

const (
	shortLinkCounterKey = "shortlinkCount"
	shortLinksKey       = "shortLinks"
)

func (d *dataAccess) ShortenLink(fullURL string) (id int, err error) {
	conn := d.Get()
	defer conn.Close()

	newID, err := redis.Int(conn.Do("INCR", shortLinkCounterKey))
	if err != nil {
		return 0, slog.Wrap(err)
	}
	if _, err := conn.Do("HSET", shortLinksKey, fmt.Sprint(newID), fullURL); err != nil {
		return 0, slog.Wrap(err)
	}
	return newID, nil
}

func (d *dataAccess) GetShortLink(id int) (fullURL string, err error) {
	conn := d.Get()
	defer conn.Close()

	s, err := redis.String(conn.Do("HGET", shortLinksKey, fmt.Sprint(id)))
	if err != nil {
		return "", slog.Wrap(err)
	}
	return s, nil
}
