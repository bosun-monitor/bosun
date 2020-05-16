package database

import (
	"bosun.org/opentsdb"
)

// MetricMetadata is metadata for a metric
type MetricMetadata struct {
	Desc        string `redis:"desc" json:",omitempty"`
	Unit        string `redis:"unit" json:",omitempty"`
	Rate        string `redis:"rate" json:",omitempty"`
	LastTouched int64  `redis:"lastTouched"`
}

// TagMetadata is metadata for a tag
type TagMetadata struct {
	Tags        opentsdb.TagSet
	Name        string
	Value       string
	LastTouched int64
}

// LastInfo is the last value and the change to its previous value
type LastInfo struct {
	LastVal      float64
	DiffFromPrev float64
	Timestamp    int64
}
