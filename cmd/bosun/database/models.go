package database

import ()

type MetricMetadata struct {
	Desc        string `redis:"desc"`
	Unit        string `redis:"unit"`
	Type        string `redis:"type"`
	LastTouched int64  `redis:"lastTouched"`
}
