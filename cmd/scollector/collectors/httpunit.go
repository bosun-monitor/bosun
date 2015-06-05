package collectors

import (
	"fmt"

	"bosun.org/_third_party/github.com/BurntSushi/toml"
	"bosun.org/_third_party/github.com/StackExchange/httpunit"
	"bosun.org/metadata"
	"bosun.org/opentsdb"
)

func HTTPUnitFile(filename string) error {
	var plans httpunit.Plans
	if _, err := toml.DecodeFile(filename, &plans); err != nil {
		return err
	}
	HTTPUnitPlans(filename, &plans)
	return nil
}

func HTTPUnitPlans(name string, plans *httpunit.Plans) {
	collectors = append(collectors, &IntervalCollector{
		F: func() (opentsdb.MultiDataPoint, error) {
			return cHTTPUnit(plans)
		},
		name: fmt.Sprintf("c_httpunit_%s", name),
	})
}

func cHTTPUnit(plans *httpunit.Plans) (opentsdb.MultiDataPoint, error) {
	ch, _, err := plans.Test("", false)
	if err != nil {
		return nil, err
	}
	var md opentsdb.MultiDataPoint
	for r := range ch {
		tags := opentsdb.TagSet{
			"protocol":     r.Case.URL.Scheme,
			"ip":           r.Case.IP.String(),
			"url_host":     r.Case.URL.Host,
			"hc_test_case": r.Plan.Label,
		}
		var er = 0
		if r.Result.Result != nil {
			er = 1
		}
		Add(&md, "hu.error", er, tags, metadata.Gauge, metadata.Bool, "")
		var sc = 0
		if r.Result.Connected {
			sc = 1
		}
		Add(&md, "hu.socket_connected", sc, tags, metadata.Gauge, metadata.Bool, descHTTPUnitSocketConnected)
		switch r.Case.URL.Scheme {
		case "http", "https":
			Add(&md, "hu.http.got_expected_code", r.Result.GotCode, tags, metadata.Gauge, metadata.Bool, descHTTPUnitExpectedCode)
			Add(&md, "hu.http.got_expected_text", r.Result.GotText, tags, metadata.Gauge, metadata.Bool, descHTTPUnitExpectedText)
			Add(&md, "hu.http.got_expected_regex", r.Result.GotRegex, tags, metadata.Gauge, metadata.Bool, descHTTPUnitExpectedRegex)
			if r.Case.URL.Scheme == "https" {
				cv := 1
				if r.Result.InvalidCert {
					cv = 0
				}
				Add(&md, "hu.cert.valid", cv, tags, metadata.Gauge, metadata.Bool, "")
				Add(&md, "hu.cert.expires", r.Result.Resp.TLS.PeerCertificates[0].NotAfter.Unix(), tags, metadata.Gauge, metadata.Timestamp, "")
			}
		}
	}
	return md, nil
}

const (
	descHTTPUnitError           = "1 if any error (no connection, unexpected code or text) occurred, else 0."
	descHTTPUnitSocketConnected = "1 if a connection was made, else 0."
	descHTTPUnitExpectedCode    = "1 if the HTTP status code was expected, else 0."
	descHTTPUnitExpectedText    = "1 if the response contained expected text, else 0."
	descHTTPUnitExpectedRegex   = "1 if the response matched expected regex, else 0."
	descHTTPUnitCertValid       = "1 if the SSL certificate is valid, else 0."
	descHTTPUnitCertExpires     = "Unix epoch time of the certificate expiration."
)
