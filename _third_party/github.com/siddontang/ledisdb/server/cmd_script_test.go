// +build lua

package server

import (
	"fmt"
	"reflect"
	"testing"

	"bosun.org/_third_party/github.com/siddontang/goredis"
)

func TestCmdEval(t *testing.T) {
	c := getTestConn()
	defer c.Close()

	if v, err := goredis.Strings(c.Do("eval", "return {KEYS[1],KEYS[2],ARGV[1],ARGV[2]}", 2, "key1", "key2", "first", "second")); err != nil {
		t.Fatal(err)
	} else if !reflect.DeepEqual(v, []string{"key1", "key2", "first", "second"}) {
		t.Fatal(fmt.Sprintf("%v", v))
	}

	if v, err := goredis.Strings(c.Do("eval", "return {KEYS[1],KEYS[2],ARGV[1],ARGV[2]}", 2, "key1", "key2", "first", "second")); err != nil {
		t.Fatal(err)
	} else if !reflect.DeepEqual(v, []string{"key1", "key2", "first", "second"}) {
		t.Fatal(fmt.Sprintf("%v", v))
	}

	var sha1 string
	var err error
	if sha1, err = goredis.String(c.Do("script", "load", "return {KEYS[1],KEYS[2],ARGV[1],ARGV[2]}")); err != nil {
		t.Fatal(err)
	} else if len(sha1) != 40 {
		t.Fatal(sha1)
	}

	if v, err := goredis.Strings(c.Do("evalsha", sha1, 2, "key1", "key2", "first", "second")); err != nil {
		t.Fatal(err)
	} else if !reflect.DeepEqual(v, []string{"key1", "key2", "first", "second"}) {
		t.Fatal(fmt.Sprintf("%v", v))
	}

	if ay, err := goredis.Values(c.Do("script", "exists", sha1, "01234567890123456789")); err != nil {
		t.Fatal(err)
	} else if !reflect.DeepEqual(ay, []interface{}{int64(1), int64(0)}) {
		t.Fatal(fmt.Sprintf("%v", ay))
	}

	if ok, err := goredis.String(c.Do("script", "flush")); err != nil {
		t.Fatal(err)
	} else if ok != "OK" {
		t.Fatal(ok)
	}

	if ay, err := goredis.Values(c.Do("script", "exists", sha1)); err != nil {
		t.Fatal(err)
	} else if !reflect.DeepEqual(ay, []interface{}{int64(0)}) {
		t.Fatal(fmt.Sprintf("%v", ay))
	}
}
