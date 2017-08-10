package statusio

import "testing"

func TestExtractScheme(t *testing.T) {
	samples := []struct {
		url      string
		scheme   string
		baseAddr string
	}{
		// url      =>             scheme, base address
		{"foo.com", "https", "foo.com"},
		{"http://foo.com", "http", "foo.com"},
		{"https://foo.com", "https", "foo.com"},
		{"http://https://foo.com", "http", "https:"},
		{"", "https", ""},
	}
	for _, s := range samples {
		scheme, baseAddr := extractScheme(s.url)
		if scheme != s.scheme {
			t.Errorf("scheme: got %s, wants: %s (%s)",
				scheme, s.scheme, s.url)
		}
		if baseAddr != s.baseAddr {
			t.Errorf("baseAddr: got %s, wants: %s (%s)",
				baseAddr, s.baseAddr, s.url)
		}
	}
}
