package database

import (
	"bosun.org/opentsdb"
	"fmt"
	"time"
)

/*
	Tag metadata gets stored in various ways:

	Metadata itself gets stored as a hash (tmeta:{tags}:name).
	Fields are simply value, and updated (unix timestamp)

	To facilitate subset lookups, there will be index sets for each subset of inserted tags.
	tmetaidx:{subset} -> set of tmeta keys
*/

func tagMetaKey(tags opentsdb.TagSet, name string) string {
	return fmt.Sprintf("tmeta:%s:%s", tags.Tags(), name)
}

func (d *dataAccess) PutTagMetadata(tags opentsdb.TagSet, name string, value string, updated time.Time) error {
	conn := d.getConnection()
	key := tagMetaKey(tags, name)
	_, err := conn.Do("HMSET", key, "value", value, "updated", updated.UTC().Unix())
	if err != nil {
		return err
	}
	for _, sub := range tags.AllSubsets() {
		subkey := fmt.Sprintf("tmetaidx:%s", sub)
		_, err := conn.Do("SADD", subkey, key)
		if err != nil {
			return err
		}
	}
	return nil
}
