package dbtest

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"bosun.org/cmd/bosun/database"
)

var flagReddisHost = flag.String("redis", "", "redis server to test against")
var flagFlushRedis = flag.Bool("flush", false, "flush database before tests. DANGER!")

func StartTestRedis() (database.DataAccess, func()) {
	flag.Parse()
	// For redis tests we just point at an external server.
	if *flagReddisHost != "" {
		testData := database.NewDataAccess(*flagReddisHost, true, 0, "")
		if *flagFlushRedis {
			log.Println("FLUSHING REDIS")
			c := testData.(database.Connector).GetConnection()
			defer c.Close()
			_, err := c.Do("FLUSHDB")
			if err != nil {
				log.Fatal(err)
			}
		}
		return testData, func() {}
	}
	// To test ledis, start a local instance in a new tmp dir. We will attempt to delete it when we're done.
	addr := "127.0.0.1:9876"
	testPath := filepath.Join(os.TempDir(), "bosun_ledis_test", fmt.Sprint(time.Now().Unix()))
	log.Println(testPath)
	stop, err := database.StartLedis(testPath, addr)
	if err != nil {
		log.Fatal(err)
	}
	testData := database.NewDataAccess(addr, false, 0, "")
	return testData, func() {
		stop()
		os.RemoveAll(testPath)
	}
}
