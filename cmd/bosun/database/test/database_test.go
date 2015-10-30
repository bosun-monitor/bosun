package dbtest

import (
	"math/rand"
	"os"
	"testing"
	"time"

	"bosun.org/cmd/bosun/database"
)

// data access object to use for all unit tests. Pointed at ephemeral ledis, or redis server passed in with --redis=addr
var testData database.DataAccess

func TestMain(m *testing.M) {
	rand.Seed(time.Now().UnixNano())
	var closeF func()
	testData, closeF = StartTestRedis()
	status := m.Run()
	closeF()
	os.Exit(status)
}

var cleanups = []func(){}

// use random keys in tests to avoid conflicting test data.
func randString(l int) string {
	s := ""
	for len(s) < l {
		s += string("abcdefghijklmnopqrstuvwxyz"[rand.Intn(26)])
	}
	return s
}

func check(t *testing.T, err error) {
	if err != nil {
		t.Fatal(err)
	}
}
