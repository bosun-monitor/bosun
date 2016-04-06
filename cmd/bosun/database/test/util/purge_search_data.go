package main

import (
	"flag"
	"fmt"

	"bosun.org/cmd/bosun/database"
)

// USAGE: go run purge_search_data.go -r redishost:6379 -m metricIWantToClear
// use -n to see what would get deleted, without changing any data

var redis = flag.String("r", "", "redis host:port")
var metric = flag.String("m", "", "metric to purge")
var noop = flag.Bool("n", false, "only print commands, don't run.")

func main() {
	flag.Parse()
	if *redis == "" || *metric == "" {
		flag.PrintDefaults()
		return
	}
	db := database.NewDataAccess(*redis, true, 0, "").(interface {
		PurgeSearchData(string, bool) error
	})
	err := db.PurgeSearchData(*metric, *noop)
	fmt.Println(err)
}
