package collectors

import (
	"fmt"
	"time"

	"bosun.org/slog"

	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"github.com/BurntSushi/toml"
	"github.com/StackExchange/httpunit"
)

func HTTPUnitTOML(filename string, freq time.Duration) error {
	var plans httpunit.Plans
	if _, err := toml.DecodeFile(filename, &plans); err != nil {
		return err
	}
	HTTPUnitPlans(filename, &plans, freq)
	return nil
}

func HTTPUnitHiera(filename string, freq time.Duration) error {
	plans, err := httpunit.ExtractHiera(filename)
	if err != nil {
		return err
	}
	HTTPUnitPlans(filename, &httpunit.Plans{Plans: plans}, freq)
	return nil
}

func HTTPUnitPlans(name string, plans *httpunit.Plans, freq time.Duration) {
	collectors = append(collectors, &IntervalCollector{
		F: func() (opentsdb.MultiDataPoint, error) {
			return cHTTPUnit(plans)
		},
		name:     fmt.Sprintf("c_httpunit_%s", name),
		Interval: freq,
	})
}

func cHTTPUnit(plans *httpunit.Plans) (opentsdb.MultiDataPoint, error) {
	ch, _, err := plans.Test("", false)
	if err != nil {
		return nil, err
	}
	unix_now := time.Now().Unix()
	var md opentsdb.MultiDataPoint
	for r := range ch {
		if r == nil || r.Case == nil || r.Plan == nil {
			slog.Errorf("httpunit: unexpected test plan on channel, the plan result, the case, and/or the plan result is missing.")
			continue
		}
		tags := opentsdb.TagSet{
			"protocol":     r.Case.URL.Scheme,
			"ip":           r.Case.IP.String(),
			"url_host":     r.Case.URL.Host,
			"hc_test_case": r.Plan.Label,
		}
		metadata.AddMeta("", opentsdb.TagSet{"hc_test_case": r.Plan.Label}, "url", r.Case.URL.String(), false)
		if r.Result == nil || r.Result.Result != nil {
			Add(&md, "hu.error", true, tags, metadata.Gauge, metadata.Bool, descHTTPUnitError)
			if r.Result == nil {
				continue
			}
		}
		Add(&md, "hu.error", false, tags, metadata.Gauge, metadata.Bool, descHTTPUnitError)
		ms := int64(r.Result.TimeTotal / time.Millisecond)
		Add(&md, "hu.socket_connected", r.Result.Connected, tags, metadata.Gauge, metadata.Bool, descHTTPUnitSocketConnected)
		Add(&md, "hu.time_total", ms, tags, metadata.Gauge, metadata.MilliSecond, descHTTPUnitTotalTime)
		code := 0
		if r.Result.Resp != nil {
			code = r.Result.Resp.StatusCode
		}
		Add(&md, "hu.response_code", code, tags, metadata.Gauge, metadata.StatusCode, descHTTPUnitStatusCode)
		switch r.Case.URL.Scheme {
		case "http", "https":
			Add(&md, "hu.http.got_expected_code", r.Result.GotCode, tags, metadata.Gauge, metadata.Bool, descHTTPUnitExpectedCode)
			Add(&md, "hu.http.got_expected_text", r.Result.GotText, tags, metadata.Gauge, metadata.Bool, descHTTPUnitExpectedText)
			Add(&md, "hu.http.got_expected_regex", r.Result.GotRegex, tags, metadata.Gauge, metadata.Bool, descHTTPUnitExpectedRegex)
			if r.Case.URL.Scheme == "https" {
				Add(&md, "hu.cert.valid", !r.Result.InvalidCert, tags, metadata.Gauge, metadata.Bool, descHTTPUnitCertValid)
				if resp := r.Result.Resp; resp != nil && resp.TLS != nil && len(resp.TLS.PeerCertificates) > 0 {
					expiration := resp.TLS.PeerCertificates[0].NotAfter.Unix()
					Add(&md, "hu.cert.expires", expiration, tags, metadata.Gauge, metadata.Timestamp, descHTTPUnitCertExpires)
					Add(&md, "hu.cert.valid_for", expiration-unix_now, tags, metadata.Gauge, metadata.Second, descHTTPUnitCertValidFor)
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
	descHTTPUnitCertValidFor    = "Number of seconds until certificate expiration."
	descHTTPUnitTotalTime       = "Total time consumed by test case."
	descHTTPUnitStatusCode      = "The HTTP status response code of the response, or 0 if no response."
)
