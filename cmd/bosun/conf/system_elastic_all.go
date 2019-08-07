package conf

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"bosun.org/cmd/bosun/expr"
	"bosun.org/slog"
)

// ParseESConfig return expr.ElasticHost
func parseESConfig(sc *SystemConf) expr.ElasticHosts {
	store := make(map[string]expr.ElasticConfig)
	for hostPrefix, value := range sc.ElasticConf {
		// build es config per cluster
		version := getESVersion(hostPrefix, value)
		var cfg expr.ElasticConfig
		switch expr.ESVersion(version) {
		case expr.ESV2:
			cfg = parseESConfig2(value)
		case expr.ESV5:
			cfg = parseESConfig5(value)
		case expr.ESV6:
			cfg = parseESConfig6(value)
		case expr.ESV7:
			cfg = parseESConfig7(value)
		case "":
			slog.Fatal(fmt.Errorf(`conf: [ElasticConf.%s]: Version is required a field (supported values for Version are: "v2", "v5", "v6" and "v7")`, hostPrefix))
		default:
			slog.Fatal(fmt.Errorf(`conf: [ElasticConf.%s]: invalid elastic version: %s (supported versions are: "v2", "v5", "v6" and "v7")`, hostPrefix, value.Version))
		}
		cfg.Version = expr.ESVersion(version)
		store[hostPrefix] = cfg
	}

	return expr.ElasticHosts{Hosts: store}
}

// ParseESConfig return expr.ElasticHost
func parseESAnnoteConfig(sc *SystemConf) expr.ElasticConfig {
	var cfg expr.ElasticConfig
	if len(sc.AnnotateConf.Hosts) == 0 {
		return cfg
	}
	switch expr.ESVersion(sc.AnnotateConf.Version) {
	case expr.ESV2:
		cfg = parseESConfig2(ElasticConf(sc.AnnotateConf))
	case expr.ESV5:
		cfg = parseESConfig5(ElasticConf(sc.AnnotateConf))
	case expr.ESV6:
		cfg = parseESConfig6(ElasticConf(sc.AnnotateConf))
	case expr.ESV7:
		cfg = parseESConfig7(ElasticConf(sc.AnnotateConf))
	case "":
		slog.Fatal(fmt.Errorf(`conf: [AnnotateConf]: Version is required a field (supported values for Version are: "v2", "v5", "v6" and "v7")`))
	default:
		slog.Fatal(fmt.Errorf(`conf: [AnnotateConf]: invalid elastic version: %s (supported versions are: "v2", "v5", "v6" and "v7")`, sc.AnnotateConf.Version))
	}

	cfg.Version = expr.ESVersion(sc.AnnotateConf.Version)
	return cfg
}

// getESVersion queries ES version and use "Version" value defined in configuration
// as the fallback value
func getESVersion(hostPrefix string, esConf ElasticConf) string {
	version := esConf.Version
	if esConf.RuntimeVersionEnabled {
		var err error
		for _, h := range esConf.Hosts {
			version, err = queryESVersion(esConf.RuntimeBasicAuthUsername, esConf.RuntimeBasicAuthPassword, h)
			if err != nil {
				slog.Error(fmt.Errorf("conf: [ElasticConf.%s]: invalid query elastic version result for host %s, error: %s", hostPrefix, h, err))
			} else {
				break
			}
		}
	}
	return version
}

// esVersionResp respresents an ES response
// Only first level keys are defined to to make it more resilient to response struct changes
type esVersionResp struct {
	Name         string
	Cluster_name string
	Cluster_uuid string
	Version      map[string]interface{}
	Tagline      string
}

// queryESVersion sends request and parses response
func queryESVersion(username string, password string, url string) (string, error) {
	client := &http.Client{
		Timeout: time.Second * 3,
	}
	req, err := http.NewRequest("GET", url, nil)
	req.Header.Add("Content-Type", "application/json")
	req.SetBasicAuth(username, password)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result esVersionResp
	var version string
	err = json.Unmarshal([]byte(body), &result)
	if err != nil {
		return "", err
	}

	if value, ok := result.Version["number"]; ok {
		if val, ok := value.(string); ok {
			nums := strings.Split(val, ".")
			version = "v" + nums[0]
			return version, nil
		}
	}
	return "", fmt.Errorf("unable to parse version from ES response: %v", result.Version)
}
