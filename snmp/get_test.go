// +build ignore

package snmp

import (
	"fmt"
	"strings"
	"testing"
)

type GetTest struct {
	oid    string
	expect interface{}
}

var getTests = []GetTest{
	{stringType, []byte(nil)},
	{oidType, []int(nil)},
	{timeticksType, int64(0)},
	{counter32Type, int64(0)},
	{counter64Type, int64(0)},
	{gauge32Type, int64(0)},
}

func TestGetNil(t *testing.T) {
	var v interface{}
	for _, test := range getTests {
		if err := Get("localhost", "public", test.oid, &v); err != nil {
			t.Errorf("%s unexpected error: %v", test.oid, err)
			continue
		}
		have := fmt.Sprintf("%T", v)
		want := fmt.Sprintf("%T", test.expect)
		if have != want {
			t.Errorf("%s bad type, want=%s, have=%s", test.oid, want, have)
		}
	}
}

type HostTest struct {
	host   string
	expect string
}

var hostTests = []string{
	"localhost",
	"localhost:161",
}

func TestHostPort(t *testing.T) {
	var want string
	for i, host := range hostTests {
		var v []byte
		if err := Get(host, "public", stringType, &v); err != nil {
			t.Errorf("%s unexpected error: %v", host, err)
			continue
		}
		if have := string(v); i == 0 {
			want = have
		} else if have != want {
			t.Errorf("%s wrong host, want=%s, have=%s", host, want, have)
		}
	}
}

type CommunityTest struct {
	str string
	ok  bool
	err error
}

var communityTests = []CommunityTest{
	{str: "", ok: false},
	{str: "public", ok: true},
	{str: "invalid", ok: false},
}

func TestCommunity(t *testing.T) {
	ch := make(chan CommunityTest)
	for _, test := range communityTests {
		str := test.str
		test := test
		go func() {
			var v interface{}
			test.err = Get("localhost", str, stringType, &v)
			ch <- test
		}()
	}
	for range communityTests {
		test := <-ch
		if (test.err == nil) != test.ok {
			t.Errorf("%s invalid err: %s", test.str, test.err)
		}
	}
}

type TestRun struct {
	req *Request
}

func TestParallel(t *testing.T) {
	const N = 1000
	done := make(chan bool)
	for i := 0; i < N; i++ {
		go func() {
			var v interface{}
			if err := Get("localhost", "public", stringType, &v); err != nil {
				t.Errorf("%d unexpected error: %v", i, err)
			}
			done <- true
		}()
	}
	for i := 0; i < N; i++ {
		<-done
	}
}

type MultiTest struct {
	args   []interface{}
	expect string
}

var outInt int

var multiTests = []MultiTest{
	{[]interface{}{}, ""},
	{[]interface{}{"IF-MIB::ifMtu.1", &outInt}, ""},
	{[]interface{}{"IF-MIB::ifMtu.1", &outInt, "IF-MIB::ifSpeed.1", &outInt}, ""},
}

func TestGetMulti(t *testing.T) {
	for _, test := range multiTests {
		err := Get("localhost", "public", test.args...)
		if (test.expect == "") != (err == nil) {
			t.Errorf("%v: unexpected error: %v", test.args, err)
			continue
		}
		if err == nil {
			continue
		}
		if i := strings.Index(err.Error(), test.expect); i < 0 {
			t.Errorf("%v: unexpected error: %v", test.args, err)
		}
	}
}
