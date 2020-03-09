package host

import (
	"strings"
	"testing"
)

func validateManager(m Manager, expectedConfiguredHostname, expectedRealHostname string, t *testing.T) {
	expectedConfiguredHostname = strings.ToLower(expectedConfiguredHostname)
	expectedRealHostname = strings.ToLower(expectedRealHostname)

	if m.GetHostName() != expectedConfiguredHostname {
		t.Errorf("Expected hostname to be '%s' but it was '%s'", expectedConfiguredHostname, m.GetHostName())
	} else if m.GetHost().GetName() != expectedConfiguredHostname {
		t.Errorf("Expected hostname to be '%s' but it was '%s'", expectedConfiguredHostname, m.GetHostName())
	} else if m.GetNameProcessor() == nil {
		t.Errorf("Expected managed host to have a name processor")
	}
	if m.GetRealHostName() != expectedRealHostname {
		t.Errorf("Expected real hostname to be '%s' but it was '%s'", expectedRealHostname, m.GetRealHostName())
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
		hostnameMethod = func() (string, error) { return params.hostname, nil }
		m, err := NewManager(params.preserveFullHostName)
		if err != nil {
			t.Errorf("Error while create new manager for %s: %v", params.hostname, err)
		}

		validateManager(m, params.expected, params.expected, t)
	}

}

func TestNewManagerForHostname(t *testing.T) {
	testHostnames := []struct {
		configuredHostname         string
		realHostname               string
		expectedConfiguredHostname string
		expectedRealHostname       string
		preserveFullHostName       bool
	}{
		{"JiMMYs-PC", "JiMMYs-PC-2", "JiMMYs-PC", "JiMMYs-PC-2", false},
		{"JiMMYs-PC", "JiMMYs-PC-2", "JiMMYs-PC", "jimmys-pc-2", true},
		{"test1.domain.com", "host2.domain.com", "test1.domain.com", "host2.domain.com", true},
		{"test1.domain.com", "host2.domain.com", "test1", "host2", false},
		{"test-01.domain.com", "host-10.domain.com", "test-01", "host-10", false},
		{"test-01.domain.com", "host-100.domain.com", "test-01.domain.com", "host-100.domain.com", true},
		{"test-103.subdomain1.subdomain2.domain.com", "host-00001.subdomain1.domain.com", "test-103", "host-00001", false},
		{"test-103.subdomain1.subdomain2.domain.com", "host-00001.subdomain1.domain.com", "test-103.subdomain1.subdomain2.domain.com", "host-00001.subdomain1.domain.com", true},
	}
	for _, params := range testHostnames {
		hostnameMethod = func() (string, error) { return params.realHostname, nil }
		m, err := NewManagerForHostname(params.configuredHostname, params.preserveFullHostName)
		if err != nil {
			t.Errorf("Error while create new manager for hostname for %s: %v", params.configuredHostname, err)
		}

		validateManager(m, params.expectedConfiguredHostname, params.expectedRealHostname, t)
	}
}
