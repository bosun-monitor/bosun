package host

import (
	"os"
	"strings"
	"testing"
)

func validateManager(m Manager, expectedHostname string, t *testing.T) {
	expectedHostname = strings.ToLower(expectedHostname)

	if m.GetHostName() != expectedHostname {
		t.Errorf("Expected hostname to be '%s' but it was '%s'", expectedHostname, m.GetHostName())
	} else if m.GetHost().GetName() != expectedHostname {
		t.Errorf("Expected hostname to be '%s' but it was '%s'", expectedHostname, m.GetHostName())
	} else if m.GetNameProcessor() == nil {
		t.Errorf("Expected managed host to have a name processor")
	}
}

func TestNewManager(t *testing.T) {
	// we should save fqdn for NewManager since os.Hostname will return /proc/sys/kernel/hostname and it can be fqdn
	// underhood NewManager will use same call, but if we don't save fqdn we are trying to split that name
	// In that can this test will fail anyway
	m, err := NewManager(true)
	if err != nil {
		t.Error(err)
	}

	expectedHostname, err := os.Hostname()
	if err != nil {
		t.Error(err)
	}

	validateManager(m, expectedHostname, t)
}

func TestNewManagerForHostname(t *testing.T) {
	expectedHostname := "JiMMYs-PC"
	m, err := NewManagerForHostname(expectedHostname, false)
	if err != nil {
		t.Error(err)
	}

	validateManager(m, expectedHostname, t)
}
