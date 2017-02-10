package redisStore

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/garyburd/redigo/redis"

	"fmt"
	"github.com/captncraig/easyauth/providers/token"
)

type store struct {
	Connector
}

//Connector is a simple interface to retreive a connection.
//This is usually implemented by redis.Pool, but could be any method to get a connection you wish
type Connector interface {
	Get() redis.Conn
}

func New(db Connector) token.TokenDataAccess {
	return &store{
		Connector: db,
	}
}

const (
	accessTokensKey = "accessTokens"
	timestampsKey   = "accessTokenTimestamps"
)

func (s *store) LookupToken(hash string) (*token.Token, error) {
	conn := s.Get()
	defer conn.Close()

	val, err := redis.String(conn.Do("HGET", accessTokensKey, hash))
	if err != nil {
		if err == redis.ErrNil {
			return nil, fmt.Errorf("Token not found")
		}
		return nil, err
	}
	tok := &token.Token{}
	if err = json.Unmarshal([]byte(val), tok); err != nil {
		return nil, err
	}
	conn.Do("HSET", timestampsKey, hash, time.Now().UTC().Unix()) //eat failure here
	return tok, nil
}

func (s *store) StoreToken(t *token.Token) error {
	conn := s.Get()
	defer conn.Close()

	data, err := json.Marshal(t)
	if err != nil {
		return err
	}
	_, err = conn.Do("HSET", accessTokensKey, t.Hash, string(data))
	return err
}

func (s *store) RevokeToken(hash string) error {
	conn := s.Get()
	defer conn.Close()

	_, err := conn.Do("HDEL", accessTokensKey, hash)
	return err
}

func (s *store) ListTokens() ([]*token.Token, error) {
	conn := s.Get()
	defer conn.Close()

	tokens, err := redis.StringMap(conn.Do("HGETALL", accessTokensKey))
	if err != nil {
		return nil, err
	}
	timestamps, err := redis.StringMap(conn.Do("HGETALL", timestampsKey))
	if err != nil {
		return nil, err
	}

	toks := make([]*token.Token, 0, len(tokens))
	for _, tok := range tokens {
		t := &token.Token{}
		if err = json.Unmarshal([]byte(tok), t); err != nil {
			return nil, err
		}
		ts, ok := timestamps[t.Hash]
		if ok {
			if tsi, err := strconv.ParseInt(ts, 10, 64); err == nil {
				t.LastUsed = time.Unix(tsi, 0).UTC()
			}
		}
		toks = append(toks, t)
	}
	return toks, nil
}
