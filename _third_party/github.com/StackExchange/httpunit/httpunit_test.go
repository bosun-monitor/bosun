package httpunit

import "testing"

func TestIPMapResolve(t *testing.T) {
	m := IPMap{
		"BASEIP":   []string{"87.65.43."},
		`^(\d+)$`:  []string{"*", "BASEIP$1", "BASEIP($1+64)", "123.45.67.$1"},
		`^(\d+)i$`: []string{"*", "10.0.1.$1", "10.0.2.$1", "BASEIP$1", "BASEIP($1+64)", "123.45.67.$1"},
	}
	tests := []struct {
		in    string
		error bool
		out   []string
	}{
		{
			"16",
			false,
			[]string{
				"*",
				"123.45.67.16",
				"87.65.43.16",
				"87.65.43.80",
			},
		},
		{
			"16i",
			false,
			[]string{"*",
				"10.0.1.16",
				"10.0.2.16",
				"123.45.67.16",
				"87.65.43.16",
				"87.65.43.80",
			},
		},
		{
			"unused",
			true,
			nil,
		},
	}
	for i, test := range tests {
		ips, err := m.Expand(test.in)
		if err != nil && !test.error {
			t.Errorf("%v: expected no error, got %v", i, err)
		} else if err == nil && test.error {
			t.Errorf("%v: expected error", i)
		} else if !strEqual(ips, test.out) {
			t.Errorf("%v: expected %v, got %v", i, test.out, ips)
		}
	}
}
