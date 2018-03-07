package collectors

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"bosun.org/cmd/scollector/conf"
	"bosun.org/metadata"
	"bosun.org/opentsdb"
)

func init() {
	registerInit(func(c *conf.Conf) {
		for _, n := range c.Nexpose {
			collectors = append(collectors, &IntervalCollector{
				F: func() (opentsdb.MultiDataPoint, error) {
					return c_nexpose(n.Username, n.Password, n.Host, n.Insecure, false)
				},
				name: fmt.Sprintf("nexpose-scans-%s", n.Host),
			})
			collectors = append(collectors, &IntervalCollector{
				F: func() (opentsdb.MultiDataPoint, error) {
					return c_nexpose(n.Username, n.Password, n.Host, n.Insecure, true)
				},
				name:     fmt.Sprintf("nexpose-assets-%s", n.Host),
				Interval: time.Minute * 30,
			})
		}
	})
}

func c_nexpose(username, password, host string, insecure bool, collectAssets bool) (opentsdb.MultiDataPoint, error) {
	const (
		descScanRunning   = "Nexpose scan running."
		descScanRunTime   = "Duration scan has been running, in seconds."
		descRiskScore     = "Risk score for a given site/device."
		descVulnCount     = "Number of known vulnerabilities."
		descExploitCount  = "Number of vulnerabilities exploitable via Metasploit. Subset of vuln_count."
		descMalwareCount  = "Number of vulnerabilities susceptible to malware attacks. Subset of vuln_count."
		descAssetLastScan = "How many seconds ago this asset was last scanned."
	)
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: insecure},
	}
	client := &http.Client{Transport: tr}
	url1 := fmt.Sprintf("https://%s/api/1.1/xml", host)
	url2 := fmt.Sprintf("https://%s/api/1.2/xml", host)

	var md opentsdb.MultiDataPoint

	c := nexposeConnection{Username: username, Password: password, URLv1: url1, URLv2: url2, Host: host, Client: client}

	if err := c.login(); err != nil {
		return nil, fmt.Errorf("Login failed: %s", err)
	}

	siteSummaries, err := c.siteListing()
	if err != nil {
		return nil, err
	}

	var siteNames = map[int]string{}

	for _, site := range siteSummaries {
		siteNames[site.ID] = site.Name
		if !collectAssets {
			continue
		}
		Add(&md, "nexpose.site.risk_score", site.RiskScore, opentsdb.TagSet{"site": site.Name}, metadata.Gauge, metadata.Score, descRiskScore)
		tags := opentsdb.TagSet{"site": site.Name}

		assets, err := c.getSiteAssets(site.ID)
		if err != nil {
			return nil, err
		}
		for _, asset := range assets {
			now := time.Now()
			last_scan := now.Unix() - (asset.LastScanDate / 1000)
			var assetTags opentsdb.TagSet
			// Set the AssetName to the hostname if we have it, otherwise the IP.
			if asset.AssetName != "" {
				assetTags = opentsdb.TagSet{"name": asset.shortName()}.Merge(tags)
			} else {
				assetTags = opentsdb.TagSet{"name": asset.AssetIP}.Merge(tags)
			}
			Add(&md, "nexpose.asset.risk_score", asset.RiskScore, assetTags, metadata.Gauge, metadata.Score, descRiskScore)
			Add(&md, "nexpose.asset.vuln_count", asset.VulnCount, assetTags, metadata.Gauge, metadata.Vulnerabilities, descVulnCount)
			Add(&md, "nexpose.asset.exploit_count", asset.ExploitCount, assetTags, metadata.Gauge, metadata.Vulnerabilities, descExploitCount)
			Add(&md, "nexpose.asset.malware_count", asset.MalwareCount, assetTags, metadata.Gauge, metadata.Vulnerabilities, descMalwareCount)
			Add(&md, "nexpose.asset.last_scan", last_scan, assetTags, metadata.Gauge, metadata.Second, descAssetLastScan)

			site.VulnCount += asset.VulnCount
			site.ExploitCount += asset.ExploitCount
			site.MalwareCount += asset.MalwareCount
		}

		Add(&md, "nexpose.site.vuln_count", site.VulnCount, tags, metadata.Gauge, metadata.Vulnerabilities, descVulnCount)
		Add(&md, "nexpose.site.exploit_count", site.ExploitCount, tags, metadata.Gauge, metadata.Vulnerabilities, descExploitCount)
		Add(&md, "nexpose.site.malware_count", site.MalwareCount, tags, metadata.Gauge, metadata.Vulnerabilities, descMalwareCount)
	}

	assetGroupSummaries, err := c.assetGroupListing()
	if err != nil {
		return nil, err
	}
	for _, group := range assetGroupSummaries {
		tags := opentsdb.TagSet{"asset_group": group.Name}
		Add(&md, "nexpose.asset_group.risk_score", group.RiskScore, tags, metadata.Gauge, metadata.Score, descRiskScore)
	}

	const timeFmt = "20060102T150405"
	activeScans, err := c.scanActivity()
	if err != nil {
		return nil, err
	}
	for _, scan := range activeScans {
		t, err := time.Parse(timeFmt, scan.StartTime[0:15])
		if err != nil {
			return nil, err
		}
		runtime := int(time.Since(t).Seconds())
		if scan.Status == "running" {
			Add(&md, "nexpose.scan.running", 1, opentsdb.TagSet{"site": siteNames[scan.SiteID]}, metadata.Gauge, metadata.Bool, descScanRunning)
			Add(&md, "nexpose.scan.runtime", runtime, opentsdb.TagSet{"site": siteNames[scan.SiteID]}, metadata.Gauge, metadata.Second, descScanRunTime)
		}
	}

	return md, nil
}

type nexposeConnection struct {
	Username  string
	Password  string
	Host      string
	URLv1     string
	URLv2     string
	Client    *http.Client
	SessionID string
	Cookie    *http.Cookie
}

type apiResponse struct {
	XMLName             xml.Name
	Message             string              `xml:"Message"`
	SessionID           string              `xml:"session-id,attr"`
	SiteSummaries       []siteSummary       `xml:"SiteSummary"`
	ScanSummaries       []scanSummary       `xml:"ScanSummary"`
	AssetGroupSummaries []assetGroupSummary `xml:"AssetGroupSummary"`
	Devices             []device            `xml:"SiteDevices>device"`
}

type loginRequest struct {
	XMLName  xml.Name `xml:"LoginRequest"`
	UserID   string   `xml:"user-id,attr"`
	Password string   `xml:"password,attr"`
}

type siteRequest struct {
	XMLName   xml.Name
	SessionID string `xml:"session-id,attr"`
	SiteID    int    `xml:"site-id,attr"`
}

type simpleRequest struct {
	XMLName   xml.Name
	SessionID string `xml:"session-id,attr"`
}

func (c *nexposeConnection) jsonRequest(method string, location string, v *url.Values) ([]byte, error) {
	req, _ := http.NewRequest(method, "https://"+c.Host+location, strings.NewReader(v.Encode()))

	// Some methods require the cookie, some require the header, so we set both.
	req.AddCookie(c.Cookie)
	req.Header.Add("nexposeCCSessionID", c.SessionID)
	if method == "POST" {
		req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	}

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func (c *nexposeConnection) getJsonTable(location string, params map[string]string) (*jsonTable, error) {
	v := url.Values{}
	v.Set("dir", "ASC")
	v.Set("startIndex", "0")
	v.Set("results", "500")
	for param, value := range params {
		v.Set(param, value)
	}

	body, err := c.jsonRequest("POST", location, &v)
	if err != nil {
		return nil, err
	}

	var table jsonTable
	if err = json.Unmarshal(body, &table); err != nil {
		return nil, err
	}
	total := table.TotalRecords

	for len(table.Records) < total {
		v.Set("startIndex", fmt.Sprintf("%d", len(table.Records)))
		if body, err = c.jsonRequest("POST", location, &v); err != nil {
			return nil, err
		}
		err := table.concat(body)
		if err != nil {
			return nil, err
		}
	}
	return &table, nil
}

func (c *nexposeConnection) xmlRequest(request interface{}, version int) (*apiResponse, error) {
	buf, err := xml.Marshal(&request)
	if err != nil {
		return nil, err
	}

	var url string
	if version == 1 {
		url = c.URLv1
	} else if version == 2 {
		url = c.URLv2
	} else {
		return nil, fmt.Errorf("Unsupported API version requested.")
	}

	resp, err := c.Client.Post(url, "text/xml", bytes.NewBuffer(buf))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	response, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var data apiResponse
	err = xml.Unmarshal(response, &data)
	if err != nil {
		return nil, err
	}

	if data.XMLName.Local == "Failure" {
		return nil, fmt.Errorf(data.Message)
	}

	return &data, nil
}

func (c *nexposeConnection) login() error {
	login := loginRequest{UserID: c.Username, Password: c.Password}
	resp, err := c.xmlRequest(&login, 1)
	if err != nil {
		return err
	}

	if resp.SessionID == "" {
		return fmt.Errorf("No SessionID in response.")
	}
	c.SessionID = resp.SessionID

	// Set the cookie as well, in case we want to use the undocumented JSON API.
	// Note that this cookie must not be sent to XML requests. It is only added
	// for JSON requests, thus it is not added to the client's Jar.
	c.Cookie = &http.Cookie{
		Name:   "nexposeCCSessionID",
		Value:  c.SessionID,
		Path:   "/",
		Domain: c.Host,
		Secure: true}

	return nil
}

type siteSummary struct {
	ID           int     `xml:"id,attr"`
	Name         string  `xml:"name,attr"`
	Description  string  `xml:"description,attr"`
	RiskFactor   float32 `xml:"riskfactor,attr"`
	RiskScore    float64 `xml:"riskscore,attr"`
	ExploitCount int
	MalwareCount int
	VulnCount    int
}

func (c *nexposeConnection) siteListing() ([]siteSummary, error) {
	request := simpleRequest{XMLName: xml.Name{Local: "SiteListingRequest"}, SessionID: c.SessionID}
	resp, err := c.xmlRequest(&request, 1)
	if err != nil {
		return nil, err
	}

	return resp.SiteSummaries, nil
}

type assetGroupSummary struct {
	ID        int     `xml:"id,attr"`
	Name      string  `xml:"name,attr"`
	RiskScore float64 `xml:"riskscore,attr"`
}

func (c *nexposeConnection) assetGroupListing() ([]assetGroupSummary, error) {
	request := simpleRequest{XMLName: xml.Name{Local: "AssetGroupListingRequest"}, SessionID: c.SessionID}
	resp, err := c.xmlRequest(&request, 1)
	if err != nil {
		return nil, err
	}

	return resp.AssetGroupSummaries, nil
}

type device struct {
	ID         int     `xml:"id,attr"`
	Address    string  `xml:"address,attr"`
	RiskFactor float32 `xml:"riskfactor,attr"`
	RiskScore  float64 `xml:"riskscore,attr"`
}

func (c *nexposeConnection) deviceListing(siteID int) ([]device, error) {
	request := siteRequest{XMLName: xml.Name{Local: "SiteDeviceListingRequest"}, SessionID: c.SessionID}
	if siteID > 0 {
		request.SiteID = siteID
	}
	resp, err := c.xmlRequest(&request, 1)
	if err != nil {
		return nil, err
	}

	return resp.Devices, nil
}

type scanSummary struct {
	ID        int    `xml:"scan-id,attr"`
	SiteID    int    `xml:"site-id,attr"`
	EngineID  int    `xml:"engine-id,attr"`
	Name      string `xml:"name,attr"`
	StartTime string `xml:"startTime,attr"` // %Y%M%dT%H%M%s, with 3-digit millis added to the end
	EndTime   string `xml:"endTime,attr"`   // same as above
	Status    string `xml:"status,attr"`
}

func (c *nexposeConnection) scanHistory(siteID int) ([]scanSummary, error) {
	request := siteRequest{SiteID: siteID}
	request.XMLName = xml.Name{Local: "SiteScanHistoryRequest"}
	request.SessionID = c.SessionID
	resp, err := c.xmlRequest(&request, 1)
	if err != nil {
		return nil, err
	}

	return resp.ScanSummaries, nil
}

func (c *nexposeConnection) scanActivity() ([]scanSummary, error) {
	request := simpleRequest{}
	request.XMLName = xml.Name{Local: "ScanActivityRequest"}
	request.SessionID = c.SessionID
	resp, err := c.xmlRequest(&request, 1)
	if err != nil {
		return nil, err
	}

	return resp.ScanSummaries, nil
}

func (c *nexposeConnection) getSiteAssets(siteID int) ([]asset, error) {
	params := make(map[string]string)

	params["sort"] = "assetName"
	params["table-id"] = "site-assets"
	params["siteID"] = fmt.Sprintf("%d", siteID)

	var assets []asset
	table, err := c.getJsonTable("/data/asset/site", params)
	if err != nil {
		return nil, err
	}

	for _, record := range table.Records {
		var newAsset asset
		err = json.Unmarshal(record, &newAsset)
		if err != nil {
			return nil, err
		}
		assets = append(assets, newAsset)
	}

	return assets, nil
}

type asset struct {
	AssetID      int
	AssetIP      string
	AssetName    string
	AssetOSName  string
	MacAddr      string
	RiskScore    float64
	ExploitCount int
	MalwareCount int
	VulnCount    int
	Assessed     bool
	LastScanDate int64 // 13-digit epoch with fractional seconds
	HostType     string
}

func (a *asset) shortName() string {
	if a.AssetName == "" {
		return ""
	}

	return strings.Split(a.AssetName, ".")[0]
}

type jsonTable struct {
	RowsPerPage  int
	RecordOffset int
	TotalRecords int
	Records      []json.RawMessage
}

func (t *jsonTable) concat(jsonData []byte) error {
	var newTable jsonTable
	if err := json.Unmarshal(jsonData, &newTable); err != nil {
		return err
	}
	t.Records = append(t.Records, newTable.Records...)

	return nil
}
