package database

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// data access object to use for all unit tests. Pointed at ephemeral ledis, or redis server passed in with --redis=addr
var testData *dataAccess

var flagReddisHost = flag.String("redis", "", "redis server to test against")
var flagFlushRedis = flag.Bool("flush", false, "flush database before tests. DANGER!")

func TestMain(m *testing.M) {
	flag.Parse()
	rand.Seed(time.Now().UnixNano())
	if *flagReddisHost != "" {
		testData = newDataAccess(*flagReddisHost, true)
		if *flagFlushRedis {
			log.Println("FLUSHING REDIS")
			c := testData.getConnection()
			_, err := c.Do("FLUSHDB")
			if err != nil {
				log.Fatal(err)
			}
		}
	} else {
		addr := "127.0.0.1:9876"
		testPath := filepath.Join(os.TempDir(), "bosun_ledis_test", fmt.Sprint(time.Now().Unix()))
		log.Println(testPath)
		stop, err := StartLedis(testPath, addr)
		if err != nil {
			log.Fatal(err)
		}
		testData = newDataAccess(addr, false)
		cleanups = append(cleanups, func() {
			stop()
			os.RemoveAll(testPath)
		})
	}
	status := m.Run()
	for _, c := range cleanups {
		c()
	}
	os.Exit(status)
}

var cleanups = []func(){}

func randString(l int) string {
	s := ""
	for len(s) < l {
		s += string("abcdefghijklmnopqrstuvwxyz"[rand.Intn(26)])
	}
	return s
}
