package host

import (
	"bosun.org/name"
	"testing"
)

type testCase struct {
	providedName, expectedName string
}

func getNameProcessor(useFullName bool) name.Processor {
	np, err := NewHostNameProcessor(useFullName)
	if err != nil {
		panic("Failed to create name.Processor")
	}

	return np
}

func initialiseHost(name string, useFullName bool) Host {
	h, err := NewHost(name, getNameProcessor(useFullName))
	if err != nil {
		panic(err)
	}

	return h
}

func validateTestCases(host Host, testCases []testCase, t *testing.T) {
	if host.GetNameProcessor() == nil {
		t.Error("Expected host to have a name processor")
		return
	}

	for _, tc := range testCases {
		if err := host.SetName(tc.providedName); err != nil {
			t.Error(err)
			continue
		}

		if got := host.GetName(); got != tc.expectedName {
			t.Errorf("Expected '%s' but got '%s'", tc.expectedName, got)
		}
	}
}

func TestNew_ShortName(t *testing.T) {
	testCases := []testCase{
		{"host", "host"},
		{"mach.acme.com", "mach"},
		{"abc-def.acme.co.uk", "abc-def"},
	}

	n := "mach1"
	host := initialiseHost(n+".domain.com", false)
	if host.GetName() != n {
		t.Errorf("Expected '%s' but got '%s'", n, host.GetName())
	}

	validateTestCases(host, testCases, t)
}

func TestNew_FullName(t *testing.T) {
	testCases := []testCase{
		{"host", "host"},
		{"mach.acme.com", "mach.acme.com"},
		{"abc-def.acme.co.uk", "abc-def.acme.co.uk"},
	}

	host := initialiseHost("some.name", true)
	validateTestCases(host, testCases, t)
}

func TestChangeNameProcessor(t *testing.T) {
	testCases := []testCase{
		{"host", "host"},
		{"mach.acme.com", "mach"},
		{"abc-def.acme.co.uk", "abc-def"},
	}

	host := initialiseHost("some.name", false)
	validateTestCases(host, testCases, t)

	if err := host.SetNameProcessor(getNameProcessor(true)); err != nil {
		t.Error(err)
		return
	}

	testCases = []testCase{
		{"host", "host"},
		{"mach.acme.com", "mach.acme.com"},
		{"abc-def.acme.co.uk", "abc-def.acme.co.uk"},
	}

	validateTestCases(host, testCases, t)
}
