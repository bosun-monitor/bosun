// Package mib parses modules of the virtual management information store.
package mib

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	"bosun.org/_third_party/github.com/mjibson/snmp/asn1"
)

var mibDir = ""

// Load registers an additional directory with MIB files. The standard
// system MIBs are always pre-loaded.
func Load(dir string) {
	if mibDir == "" {
		mibDir = dir
	} else {
		mibDir += ":" + dir
	}
}

// Lookup looks up the given object prefix using the snmptranslate utility.
func Lookup(prefix string) (asn1.ObjectIdentifier, error) {
	cache.Lock()
	if oid, ok := cache.lookup[prefix]; ok {
		cache.Unlock()
		return oid, nil
	}
	cache.Unlock()
	if oid, err := parseOID(prefix); err == nil {
		cache.Lock()
		cache.lookup[prefix] = oid
		cache.Unlock()
		return oid, nil
	}
	cmd := exec.Command(
		"snmptranslate",
		"-Le",
		"-M", "+"+mibDir,
		"-m", "all",
		"-On",
		prefix,
	)
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("snmp: Lookup(%q): %q: %s", prefix, cmd.Args, err)
	}
	if stderr.Len() != 0 {
		return nil, fmt.Errorf("snmp: Lookup(%q): %q: %s", prefix, cmd.Args, stderr)
	}
	oid, err := parseOID(strings.TrimSpace(stdout.String()))
	if err != nil {
		return nil, err
	}
	cache.Lock()
	cache.lookup[prefix] = oid
	cache.Unlock()
	return oid, nil
}

func init() {
	cache.lookup = make(map[string]asn1.ObjectIdentifier)
}

// cache avoids excessive use of snmptranslate.
var cache struct {
	lookup map[string]asn1.ObjectIdentifier
	sync.Mutex
}

// parseOID parses the string-encoded OID, for example the
// string "1.3.6.1.2.1.1.5.0" becomes sysName.0
func parseOID(s string) (oid asn1.ObjectIdentifier, err error) {
	if s[0] == '.' {
		s = s[1:]
	}
	var n int
	for _, elem := range strings.Split(s, ".") {
		n, err = strconv.Atoi(elem)
		if err != nil {
			return
		}
		oid = append(oid, n)
	}
	return
}
