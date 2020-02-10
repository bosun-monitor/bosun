package host

import "testing"

func TestFormatName_Short(t *testing.T) {
	testCases := make([][]string, 3)
	testCases[0] = []string{"freddy", "freddy"}
	testCases[1] = []string{"freddy.somewhere.com", "freddy"}
	testCases[2] = []string{"192.168.17.6", "192.168.17.6"}

	np, err := NewHostNameProcessor(false)
	if err != nil {
		t.Error(err)
		return
	}

	for i, tc := range testCases {
		name, err := np.FormatName(tc[0])
		if err != nil {
			t.Error(err)
		}

		if name != tc[1] {
			t.Errorf("Iteration %d: '%s' != '%s'", i, tc[1], name)
		}
	}
}

func TestFormatName_Full(t *testing.T) {
	testCases := make([]string, 3)
	testCases[0] = "freddy"
	testCases[1] = "freddy.somewhere.com"
	testCases[2] = "192.168.17.6"

	np, err := NewHostNameProcessor(true)
	if err != nil {
		t.Error(err)
		return
	}

	for i, tc := range testCases {
		name, err := np.FormatName(tc)
		if err != nil {
			t.Error(err)
		}

		if name != tc {
			t.Errorf("Iteration %d: '%s' != '%s'", i, tc, name)
		}
	}
}
