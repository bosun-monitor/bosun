package database

import (
	"bosun.org/opentsdb"
)

type MetricMetadata struct {
	Desc        string `redis:"desc" json:",omitempty"`
	Unit        string `redis:"unit" json:",omitempty"`
	Rate        string `redis:"rate" json:",omitempty"`
	LastTouched int64  `redis:"lastTouched"`
}

type TagMetadata struct {
	Tags        opentsdb.TagSet
	Name        string
	Value       string
	LastTouched int64
}

type LastInfo struct {
	LastVal      float64
	DiffFromPrev float64
	Timestamp    int64
}
