package database

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// data access object to use for all unit tests. Pointed at ephemeral ledis, or redis server passed in with --redis=addr
var testData *dataAccess

var flagReddisHost = flag.String("redis", "", "redis server to test against")

func TestMain(m *testing.M) {
	flag.Parse()
	stopF := func() {}
	if *flagReddisHost != "" {
		testData = newDataAccess(*flagReddisHost, true)
	} else {
		addr := "127.0.0.1:9876"
		testPath := filepath.Join(os.TempDir(), "bosun_ledis_test", fmt.Sprint(time.Now().Unix()))
		log.Println(testPath)
		stop, err := StartLedis(testPath, addr)
		if err != nil {
			log.Fatal(err)
		}
		testData = newDataAccess(addr, false)
		stopF = func() {
			stop()
			os.RemoveAll(testPath)
		}
	}
	status := m.Run()
	stopF()
	os.Exit(status)
}
