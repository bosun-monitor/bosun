// +build ignore

package mib

import (
	"os"
	"strings"
	"testing"

	"bosun.org/_third_party/github.com/mjibson/snmp/asn1"
)

type LookupTest struct {
	prefix string
	result asn1.ObjectIdentifier
}

var lookupTests = []LookupTest{
	{"SNMPv2-MIB::sysName.0", asn1.ObjectIdentifier{1, 3, 6, 1, 2, 1, 1, 5, 0}},
	{"1.3.6.1.2.1.1.5.0", asn1.ObjectIdentifier{1, 3, 6, 1, 2, 1, 1, 5, 0}},
	{".1.3.6.1.2.1.1.5.0", asn1.ObjectIdentifier{1, 3, 6, 1, 2, 1, 1, 5, 0}},
}

func TestLookup(t *testing.T) {
	for _, test := range lookupTests {
		result, err := Lookup(test.prefix)
		if err != nil {
			t.Errorf("Lookup(%q) error: %v", test.prefix, err)
			continue
		}
		if !result.Equal(test.result) {
			t.Errorf("Lookup(%q)", test.prefix)
			t.Errorf("  want=%v", test.result)
			t.Errorf("  have=%v", result)
		}
	}
}

type LookupError struct {
	prefix string
	expect string
}

var lookupErrors = []LookupError{
	{"", "exit status 2"},
	{"foo", "exit status 2"},
	{"sysName.0", "exit status 2"},
}

func TestLookupErrors(t *testing.T) {
	for _, test := range lookupErrors {
		_, err := Lookup(test.prefix)
		if err == nil {
			t.Errorf("expected error for %q", test.prefix)
		} else if strings.Index(err.Error(), test.expect) < 0 {
			t.Errorf("expected error with %q for %q; got %s", test.expect, test.prefix, err)
		}
	}
}

func TestCacheWarm(t *testing.T) {
	// clear the cache
	cache.lookup = make(map[string]asn1.ObjectIdentifier)

	// warm it up
	_, err := Lookup("SNMPv2-MIB::sysName.0")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}

	// break snmptranslate
	defer os.Setenv("PATH", os.Getenv("PATH"))
	os.Setenv("PATH", "")

	// simulate query
	if _, err := Lookup("SNMPv2-MIB::sysName.0"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCacheCold(t *testing.T) {
	// clear the cache
	cache.lookup = make(map[string]asn1.ObjectIdentifier)

	// break snmptranslate
	defer os.Setenv("PATH", os.Getenv("PATH"))
	os.Setenv("PATH", "")

	// simulate query
	if _, err := Lookup("SNMPv2-MIB::sysName.0"); err == nil {
		t.Errorf("unexpected success")
	}
}
