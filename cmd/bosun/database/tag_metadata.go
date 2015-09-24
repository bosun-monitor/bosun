package database

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"bosun.org/_third_party/github.com/garyburd/redigo/redis"
	"bosun.org/collect"
	"bosun.org/opentsdb"
)

/*
	Tag metadata gets stored in various ways:

	Metadata itself gets stored as a simple key (tmeta:tags:name) -> "timestamp:value".

	To facilitate subset lookups, there will be index sets for each possible subset of inserted tags.
	tmeta:idx:{subset} -> set of tmeta keys
*/

func tagMetaKey(tags opentsdb.TagSet, name string) string {
	return fmt.Sprintf("tmeta:%s:%s", tags.Tags(), name)
}

func tagMetaIdxKey(sub string) string {
	return fmt.Sprintf("tmeta:idx:%s", sub)
}

func (d *dataAccess) PutTagMetadata(tags opentsdb.TagSet, name string, value string, updated time.Time) error {
	defer collect.StartTimer("redis", opentsdb.TagSet{"op": "PutTagMeta"})()
	conn := d.getConnection()
	defer conn.Close()
	key := tagMetaKey(tags, name)
	keyValue := fmt.Sprintf("%d:%s", updated.UTC().Unix(), value)
	_, err := conn.Do("SET", key, keyValue)
	if err != nil {
		return err
	}
	for _, sub := range tags.AllSubsets() {
		_, err := conn.Do("SADD", tagMetaIdxKey(sub), key)
		if err != nil {
			return err
		}
	}
	return nil
}

func (d *dataAccess) DeleteTagMetadata(tags opentsdb.TagSet, name string) error {
	defer collect.StartTimer("redis", opentsdb.TagSet{"op": "DeleteTagMeta"})()
	conn := d.getConnection()
	defer conn.Close()
	key := tagMetaKey(tags, name)
	_, err := conn.Do("DEL", key)
	if err != nil {
		return err
	}
	for _, sub := range tags.AllSubsets() {
		_, err := conn.Do("SREM", tagMetaIdxKey(sub), key)
		if err != nil {
			return err
		}
	}
	return nil
}

func (d *dataAccess) GetTagMetadata(tags opentsdb.TagSet, name string) ([]*TagMetadata, error) {
	defer collect.StartTimer("redis", opentsdb.TagSet{"op": "GetTagMeta"})()
	conn := d.getConnection()
	defer conn.Close()
	key := tagMetaIdxKey(tags.Tags())
	keys, err := redis.Strings(conn.Do("SMEMBERS", key))
	if err != nil {
		return nil, err
	}
	args := []interface{}{}
	for _, key := range keys {
		if name == "" || strings.HasSuffix(key, ":"+name) {
			args = append(args, key)
		}
	}
	results, err := redis.Strings(conn.Do("MGET", args...)) //SHOULD WE BATCH?
	data := []*TagMetadata{}
	for i := range args {
		// break up key to get tags and name
		key := args[i].(string)[len("tmeta:"):]
		sepIdx := strings.LastIndex(key, ":")
		tags := key[:sepIdx]
		name := key[sepIdx+1:]
		tagSet, err := opentsdb.ParseTags(tags)
		if err != nil {
			return nil, err
		}
		// break up response to get time and value
		parts := strings.SplitN(results[i], ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("Expect metadata value to be `time:value`")
		}
		val := parts[1]
		time, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			return nil, err
		}
		obj := &TagMetadata{
			Tags:        tagSet,
			Name:        name,
			Value:       val,
			LastTouched: time,
		}
		data = append(data, obj)
	}
	return data, nil
}
