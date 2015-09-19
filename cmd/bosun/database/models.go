package database

import (
	"bosun.org/opentsdb"
)

type MetricMetadata struct {
	Desc        string `redis:"desc"`
	Unit        string `redis:"unit"`
	Type        string `redis:"type"`
	LastTouched int64  `redis:"lastTouched"`
}

type TagMetadata struct {
	Tags        opentsdb.TagSet
	Name        string
	Value       string
	LastTouched int64
}
