package database

import (
	"encoding/json"

	"bosun.org/cmd/bosun/web/auth"
	"bosun.org/collect"
	"bosun.org/opentsdb"
	"bosun.org/slog"
	"github.com/garyburd/redigo/redis"
)

const accessTokensKey = "accessTokens"

func (d *dataAccess) Tokens() auth.TokenDataAccess { return d }

func (d *dataAccess) LookupToken(hash string) (*auth.User, error) {
	defer collect.StartTimer("redis", opentsdb.TagSet{"op": "GetActiveSilences"})()
	conn := d.GetConnection()
	defer conn.Close()

	val, err := redis.String(conn.Do("HGET", accessTokensKey, hash))
	if err != nil && err == redis.ErrNil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	tok := &auth.Token{}
	if err = json.Unmarshal([]byte(val), tok); err != nil {
		return nil, slog.Wrap(err)
	}
	u := &auth.User{
		AuthMethod:  "token",
		Name:        tok.User,
		Permissions: tok.Perms,
	}
	return u, nil
}

func (d *dataAccess) StoreToken(t *auth.Token) error {
	defer collect.StartTimer("redis", opentsdb.TagSet{"op": "GetActiveSilences"})()
	conn := d.GetConnection()
	defer conn.Close()

	data, err := json.Marshal(t)
	if err != nil {
		return slog.Wrap(err)
	}
	_, err = conn.Do("HSET", accessTokensKey, t.Hash, string(data))
	return slog.Wrap(err)
}

func (d *dataAccess) RevokeToken(hash string) error {
	defer collect.StartTimer("redis", opentsdb.TagSet{"op": "GetActiveSilences"})()
	conn := d.GetConnection()
	defer conn.Close()
	_, err := conn.Do("HDEL", accessTokensKey, hash)
	return slog.Wrap(err)
}

func (d *dataAccess) ListTokens() ([]*auth.Token, error) {
	defer collect.StartTimer("redis", opentsdb.TagSet{"op": "GetActiveSilences"})()
	conn := d.GetConnection()
	defer conn.Close()

	tokens, err := redis.StringMap(conn.Do("HGETALL", accessTokensKey))
	if err != nil {
		return nil, slog.Wrap(err)
	}
	toks := make([]*auth.Token, 0, len(tokens))
	for _, tok := range tokens {
		t := &auth.Token{}
		if err = json.Unmarshal([]byte(tok), t); err != nil {
			return nil, err
		}
		toks = append(toks, t)
	}
	return toks, nil
}
