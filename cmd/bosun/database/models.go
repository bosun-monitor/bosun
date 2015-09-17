package database

type MetricMetadata struct {
	Desc string `redis:"desc"`
	Unit string `redis:"unit"`
	Type string `redis:"type"`
}
