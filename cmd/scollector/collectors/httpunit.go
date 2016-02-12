package collectors

import (
	"fmt"
	"time"

	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"github.com/BurntSushi/toml"
	"github.com/StackExchange/httpunit"
)

func HTTPUnitTOML(filename string) error {
	var plans httpunit.Plans
	if _, err := toml.DecodeFile(filename, &plans); err != nil {
		return err
	}
	HTTPUnitPlans(filename, &plans)
	return nil
}

func HTTPUnitHiera(filename string) error {
	plans, err := httpunit.ExtractHiera(filename)
	if err != nil {
		return err
	}
	HTTPUnitPlans(filename, &httpunit.Plans{
		Plans: plans,
	})
	return nil
}

func HTTPUnitPlans(name string, plans *httpunit.Plans) {
	collectors = append(collectors, &IntervalCollector{
		F: func() (opentsdb.MultiDataPoint, error) {
			return cHTTPUnit(plans)
		},
		name:     fmt.Sprintf("c_httpunit_%s", name),
		Interval: time.Minute * 5,
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
		Add(&md, "hu.error", r.Result.Result != nil, tags, metadata.Gauge, metadata.Bool, descHTTPUnitError)
		Add(&md, "hu.socket_connected", r.Result.Connected, tags, metadata.Gauge, metadata.Bool, descHTTPUnitSocketConnected)
		switch r.Case.URL.Scheme {
		case "http", "https":
			Add(&md, "hu.http.got_expected_code", r.Result.GotCode, tags, metadata.Gauge, metadata.Bool, descHTTPUnitExpectedCode)
			Add(&md, "hu.http.got_expected_text", r.Result.GotText, tags, metadata.Gauge, metadata.Bool, descHTTPUnitExpectedText)
			Add(&md, "hu.http.got_expected_regex", r.Result.GotRegex, tags, metadata.Gauge, metadata.Bool, descHTTPUnitExpectedRegex)
			if r.Case.URL.Scheme == "https" {
				Add(&md, "hu.cert.valid", !r.Result.InvalidCert, tags, metadata.Gauge, metadata.Bool, descHTTPUnitCertValid)
				if resp := r.Result.Resp; resp != nil && resp.TLS != nil && len(resp.TLS.PeerCertificates) > 0 {
					Add(&md, "hu.cert.expires", resp.TLS.PeerCertificates[0].NotAfter.Unix(), tags, metadata.Gauge, metadata.Timestamp, descHTTPUnitCertExpires)
				}
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
