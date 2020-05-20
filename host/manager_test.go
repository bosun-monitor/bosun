package host

import (
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
	testHostnames := []struct {
		hostname             string
		expected             string
		preserveFullHostName bool
	}{
		{"test1", "test1", true},
		{"test1", "test1", false},
		{"test1.domain.com", "test1.domain.com", true},
		{"test1.domain.com", "test1", false},
		{"test-01.domain.com", "test-01", false},
		{"test-01.domain.com", "test-01.domain.com", true},
		{"test-103.subdomain1.subdomain2.domain.com", "test-103", false},
		{"test-103.subdomain1.subdomain2.domain.com", "test-103.subdomain1.subdomain2.domain.com", true},
	}
	for _, params := range testHostnames {
		hostname = func() (string, error) { return params.hostname, nil }
		m, err := NewManager(params.preserveFullHostName)
		if err != nil {
			t.Errorf("Error while create new manager for %s: %v", params.hostname, err)
		}

		validateManager(m, params.expected, t)
	}

}

func TestNewManagerForHostname(t *testing.T) {
	expectedHostname := "JiMMYs-PC"
	m, err := NewManagerForHostname(expectedHostname, false)
	if err != nil {
		t.Error(err)
	}

	validateManager(m, expectedHostname, t)
}
