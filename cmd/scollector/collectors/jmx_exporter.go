package collectors

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"math"
	"net"
	"net/http"
	"strings"
	"time"

	"bufio"
	"bytes"

	"github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"

	"bosun.org/metadata"
	"bosun.org/opentsdb"
)

// Heavily inspired by https://github.com/influxdata/telegraf/tree/master/plugins/inputs/prometheus

func JMXExporter(url string) error {
	if url == "" {
		return fmt.Errorf("empty URL in JMXExporter")
	}
	collectors = append(collectors, &IntervalCollector{
		F: func() (opentsdb.MultiDataPoint, error) {
			return c_jmx_exporter(url)
		},
		name: fmt.Sprintf("JMXExporter-%s", url),
	})
	return nil
}

func c_jmx_exporter(url string) (opentsdb.MultiDataPoint, error) {
	var resp *http.Response

	var req, err = http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error making HTTP request to %s: %s", url, err)
	}

	req.Header = make(http.Header)

	var rt http.RoundTripper = &http.Transport{
		Dial: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).Dial,
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
		ResponseHeaderTimeout: time.Duration(3 * time.Second),
	}

	resp, err = rt.RoundTrip(req)
	if err != nil {
		return nil, fmt.Errorf("error making HTTP request to %s: %s", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s returned HTTP status %s", url, resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading body: %s", err)
	}

	md, err := c_jmx_exporter_parser(body)
	if err != nil {
		return nil, fmt.Errorf("error parsing body: %s", err)
	}

	return md, nil
}

func c_jmx_exporter_parser(buf []byte) (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	var parser expfmt.TextParser

	buf = bytes.TrimPrefix(buf, []byte("\n"))
	// Read raw data
	buffer := bytes.NewBuffer(buf)
	reader := bufio.NewReader(buffer)

	metricFamilies, err := parser.TextToMetricFamilies(reader)
	if err != nil {
		return nil, fmt.Errorf("reading text format failed: %s", err)
	}
	for metricName, mf := range metricFamilies {
		for _, m := range mf.Metric {
			var valueType metadata.RateType
			var value float64

			if mf.GetType() == io_prometheus_client.MetricType_SUMMARY {
				// summary metric
				if float64(m.GetHistogram().GetSampleCount()) > 0 {
					value = float64(m.GetSummary().GetSampleSum()) / float64(m.GetHistogram().GetSampleCount())
				}
				valueType = metadata.Unknown
			} else if mf.GetType() == io_prometheus_client.MetricType_HISTOGRAM {
				// historgram metric
				if float64(m.GetHistogram().GetSampleCount()) > 0 {
					value = float64(m.GetSummary().GetSampleSum()) / float64(m.GetHistogram().GetSampleCount())
				}
				valueType = metadata.Unknown
			} else {
				// standard metric
				valueType, value = getNameAndValueAndRateType(m)
			}

			// reading tags
			tags := makeTags(m)

			metricName = strings.Replace(metricName, "_", ".", -1)
			Add(&md, metricName, value, tags, valueType, metadata.None, "")
		}
	}
	return md, nil
}

// Get labels from metric
func makeTags(m *io_prometheus_client.Metric) map[string]string {
	result := map[string]string{}
	for _, lp := range m.Label {
		result[lp.GetName()] = lp.GetValue()
	}
	return result
}

// Get name and value from metric
func getNameAndValueAndRateType(m *io_prometheus_client.Metric) (metadata.RateType, float64) {
	if m.Gauge != nil {
		if !math.IsNaN(m.GetGauge().GetValue()) {
			return metadata.Rate, float64(m.GetGauge().GetValue())
		}
	} else if m.Counter != nil {
		if !math.IsNaN(m.GetGauge().GetValue()) {
			return metadata.Counter, float64(m.GetCounter().GetValue())
		}
	} else if m.Untyped != nil {
		if !math.IsNaN(m.GetGauge().GetValue()) {
			return metadata.Unknown, float64(m.GetUntyped().GetValue())
		}
	}
	return "", 0
}
