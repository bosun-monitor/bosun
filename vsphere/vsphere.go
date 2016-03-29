// Package vsphere provides a small set of the vSphere Web Services API.
package vsphere // import "bosun.org/vsphere"

import (
	"bytes"
	"crypto/tls"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"bosun.org/util"
)

// Vsphere holds connection state for a vSphere session.
type Vsphere struct {
	url     *url.URL
	header  http.Header
	cookies http.CookieJar
	client  *http.Client
	service *ServiceContent
}

// ServiceContent partial structure to hold basic vSphere info, including Performance Manager name.
type ServiceContent struct {
	RootFolder  string    `xml:"rootFolder"`
	ViewManager string    `xml:"viewManager,omitempty"`
	About       AboutInfo `xml:"about"`
	PerfManager string    `xml:"perfManager,omitempty"`
}

// AboutInfo holds more basic vSphere info.
type AboutInfo struct {
	Name                  string `xml:"name"`
	FullName              string `xml:"fullName"`
	Vendor                string `xml:"vendor"`
	Version               string `xml:"version"`
	Build                 string `xml:"build"`
	LocaleVersion         string `xml:"localeVersion,omitempty"`
	LocaleBuild           string `xml:"localeBuild,omitempty"`
	OsType                string `xml:"osType"`
	ProductLineId         string `xml:"productLineId"`
	ApiType               string `xml:"apiType"`
	ApiVersion            string `xml:"apiVersion"`
	InstanceUuid          string `xml:"instanceUuid,omitempty"`
	LicenseProductName    string `xml:"licenseProductName,omitempty"`
	LicenseProductVersion string `xml:"licenseProductVersion,omitempty"`
}

// Connect connects and logs in to a vSphere host.
func Connect(host, user, pwd string) (*Vsphere, error) {
	header := http.Header{
		"Content-Type": []string{`text/xml; charset=UTF-8`},
		"SOAPAction":   []string{`"urn:vim25/5.5"`},
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	v := &Vsphere{
		url: &url.URL{
			Scheme: "https",
			Host:   host,
			Path:   "/sdk",
		},
		header: header,
		client: &http.Client{
			Timeout: time.Minute,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
		cookies: jar,
	}
	service := struct {
		SC *ServiceContent `xml:"Body>RetrieveServiceContentResponse>returnval"`
	}{}
	if err := v.call(soapConnect, &service); err != nil {
		return nil, err
	}
	if service.SC.About.InstanceUuid == "" {
		return nil, fmt.Errorf("vsphere: no UUID during connect")
	}
	v.service = service.SC
	userbuf := new(bytes.Buffer)
	pwdbuf := new(bytes.Buffer)
	xml.EscapeText(userbuf, []byte(user))
	xml.EscapeText(pwdbuf, []byte(pwd))
	if err := v.call(fmt.Sprintf(soapLogin, userbuf, pwdbuf), nil); err != nil {
		return nil, err
	}
	return v, nil
}

// Result contains requested data about vSphere objects.
type Result struct {
	ID    string `xml:"obj"`
	Props []struct {
		Name string `xml:"name"`
		Val  struct {
			Type  string `xml:"type,attr"`
			Inner string `xml:",innerxml"`
		} `xml:"val"`
	} `xml:"propSet"`
}

// Info queries listed properties of all objects of the specified Type.
func (v *Vsphere) Info(Type string, properties []string) ([]*Result, error) {
	var props struct {
		Folder string `xml:"Body>RetrievePropertiesResponse>returnval>propSet>val>rootFolder"`
	}
	if err := v.call(soapRetrieveServiceInstance, &props); err != nil {
		return nil, err
	}
	var view struct {
		Session string `xml:"Body>CreateContainerViewResponse>returnval"`
	}
	for i, v := range properties {
		properties[i] = fmt.Sprintf("<pathSet>%s</pathSet>", v)
	}
	if err := v.call(fmt.Sprintf(soapCreateContainerView, props.Folder, Type), &view); err != nil {
		return nil, err
	}
	var vms struct {
		Results []*Result `xml:"Body>RetrievePropertiesResponse>returnval"`
	}
	if err := v.call(fmt.Sprintf(soapRetrieve, Type, strings.Join(properties, ""), view.Session), &vms); err != nil {
		return nil, err
	}
	return vms.Results, nil
}

// ManagedObjectReference refers to a server-side Managed Object (made for type-value information).
type ManagedObjectReference struct {
	Type  string `xml:"type,attr"`
	Value string `xml:",chardata"`
}

// PerfProviderSummary describes capabilities of a performance provider.
type PerfProviderSummary struct {
	Entity           ManagedObjectReference `xml:"entity"`
	CurrentSupported bool                   `xml:"currentSupported"` // CurrentSupported True if the entity supports real-time (current) statistics.
	SummarySupported bool                   `xml:"summarySupported"`
	RefreshRate      int                    `xml:"refreshRate,omitempty"` // RefreshRate Specifies in seconds the interval between which the system updates performance statistics.
}

// PerformanceProvider Gets capabilities of a performance provider from a vSphere server.
func (v *Vsphere) PerformanceProvider(entityType string, entity string) (*PerfProviderSummary, error) {
	var pm struct {
		ProviderSummary *PerfProviderSummary `xml:"Body>QueryPerfProviderSummaryResponse>returnval"`
	}
	body := fmt.Sprintf(soapPerfProviderSummary, v.service.PerfManager, entityType, entity)
	if err := v.call(body, &pm); err != nil {
		return nil, err
	}
	return pm.ProviderSummary, nil
}

// ElementDescription Static strings used for describing an object model string or enumeration.
type ElementDescription struct {
	Label   string `xml:"label"`
	Summary string `xml:"summary"`
	Key     string `xml:"key"`
}

// PerfCounterInfo contains metadata for a performance counter.
type PerfCounterInfo struct {
	Key            int                `xml:"key"`
	NameInfo       ElementDescription `xml:"nameInfo,typeattr"`
	GroupInfo      ElementDescription `xml:"groupInfo,typeattr"`
	UnitInfo       ElementDescription `xml:"unitInfo,typeattr"`
	RollupType     string             `xml:"rollupType"`
	StatsType      string             `xml:"statsType"`
	Level          int                `xml:"level,omitempty"`
	PerDeviceLevel int                `xml:"perDeviceLevel,omitempty"`
}

// PerfCounterInfos retrieves counter information for the specified list of counter IDs.
func (v *Vsphere) PerfCounterInfos(counters string) ([]*PerfCounterInfo, error) {
	var pcis struct {
		PerfCounters []*PerfCounterInfo `xml:"Body>QueryPerfCounterResponse>returnval"`
	}
	// counters should be prepared in form of a string of multiple <counterId>%d</counterId>
	body := fmt.Sprintf(soapQueryPerfCounters, v.service.PerfManager, counters)
	if err := v.call(body, &pcis); err != nil {
		return nil, err
	}
	return pcis.PerfCounters, nil
}

// PerfSampleInfo describes information contained in a sample collection, its timestamp, and sampling interval.
type PerfSampleInfo struct {
	Timestamp time.Time `xml:"timestamp"`
	Interval  int       `xml:"interval"`
}

// PerfMetricId describes a performance counter with a performance counter ID and an instance name.
// The instance name identifies the instance and is derived from configuration names.
// For host and virtual machine devices, the instance name is the device name.
type PerfMetricId struct {
	CounterId int    `xml:"counterId"`
	Instance  string `xml:"instance"`
}

// PerfMetricIntSeries describes integer metric values.
// The size of the array must match the size of sampleInfo in the EntityMetric that contains this series
type PerfMetricIntSeries struct {
	Id    PerfMetricId `xml:"id"`
	Value int64        `xml:"value,omitempty"`
}

// PerfEntityMetric stores metric values for a specific entity in 'normal' format.
type PerfEntityMetric struct {
	Entity     ManagedObjectReference `xml:"entity"`
	SampleInfo []PerfSampleInfo       `xml:"sampleInfo,omitempty"`
	Value      []PerfMetricIntSeries  `xml:"value,omitempty,typeattr"` //hardcoded int64
}

// PerfCountersValues Retrieves the all performance realtime metrics for the specified entity (or entities).
func (v *Vsphere) PerfCountersValues(entityType string, entity string, pm *PerfProviderSummary) (*PerfEntityMetric, error) {
	var pems struct {
		PerfCountersValues *PerfEntityMetric `xml:"Body>QueryPerfResponse>returnval"`
	}
	if !pm.CurrentSupported {
		return nil, nil
	}
	body := fmt.Sprintf(soapQueryPerf, v.service.PerfManager, entityType, entity, pm.RefreshRate)
	if err := v.call(body, &pems); err != nil {
		return nil, err
	}
	return pems.PerfCountersValues, nil
}

// Datastores returns full datastore info for a given storeKey list.
func (v *Vsphere) Datastores(storeKey map[string]string) ([]*Result, error) {
	var dss struct {
		Results []*Result `xml:"Body>RetrievePropertiesResponse>returnval"`
	}

	var stores bytes.Buffer
	for i := range storeKey {
		stores.WriteString(fmt.Sprintf("<objectSet><obj type=\"Datastore\">%s</obj><skip>false</skip></objectSet>", i))
	}

	if err := v.call(fmt.Sprintf(soapRetrieveDatastore, stores.String()), &dss); err != nil {
		return nil, err
	}
	return dss.Results, nil
}

func (v *Vsphere) call(body string, dst interface{}) error {
	buf := new(bytes.Buffer)
	buf.WriteString(soapPre)
	buf.WriteString(body)
	buf.WriteString(soapPost)
	req, err := http.NewRequest("POST", v.url.String(), buf)
	if err != nil {
		return err
	}
	for k, v := range v.header {
		req.Header[k] = v
	}
	for _, c := range v.cookies.Cookies(v.url) {
		req.AddCookie(c)
	}
	resp, err := v.client.Do(req)
	if err != nil {
		return err
	}
	v.cookies.SetCookies(v.url, resp.Cookies())
	defer resp.Body.Close()
	if resp.StatusCode == 500 {
		var e Error
		if err := xml.NewDecoder(resp.Body).Decode(&e); err != nil {
			return fmt.Errorf("vsphere: bad response, could not decode error: %v", err)
		}
		return e
	} else if resp.StatusCode != 200 {
		return fmt.Errorf("vsphere: bad status code %v", resp.Status)
	}
	if dst != nil {
		return xml.NewDecoder(resp.Body).Decode(dst)
	}
	return nil
}

// Vcenter returns vsphere vcenter hostname
func (v *Vsphere) Vcenter() string {
	if v.url == nil {
		return ""
	}
	// strip port number
	fullhostname := strings.Split(v.url.Host, ":")[0]
	return util.Clean(fullhostname)
}

// Error can be returned from any call to a Vsphere object, including Connect.
type Error struct {
	Code   string `xml:"Body>Fault>faultcode"`
	String string `xml:"Body>Fault>faultstring"`
	Detail struct {
		Inner string `xml:",innerxml"`
	} `xml:"Body>Fault>detail"`
}

func (e Error) Error() string {
	if e.String != "" {
		return e.String
	}
	return e.Detail.Inner
}

const (
	soapPre  = `<?xml version="1.0" encoding="UTF-8"?><soapenv:Envelope xmlns:soapenc="http://schemas.xmlsoap.org/soap/encoding/" xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/" xmlns:xsd="http://www.w3.org/2001/XMLSchema" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"><soapenv:Body>`
	soapPost = `</soapenv:Body></soapenv:Envelope>`

	soapConnect                 = `<RetrieveServiceContent xmlns="urn:vim25"><_this type="ServiceInstance">ServiceInstance</_this></RetrieveServiceContent>`
	soapLogin                   = `<Login xmlns="urn:vim25"><_this type="SessionManager">SessionManager</_this><userName>%s</userName><password>%s</password></Login>`
	soapRetrieveServiceInstance = `<RetrieveProperties xmlns="urn:vim25"><_this type="PropertyCollector">propertyCollector</_this><specSet><propSet><type>ServiceInstance</type><all>false</all><pathSet>content</pathSet></propSet><objectSet><obj type="ServiceInstance">ServiceInstance</obj><skip>false</skip></objectSet></specSet></RetrieveProperties>`
	soapCreateContainerView     = `<CreateContainerView xmlns="urn:vim25"><_this type="ViewManager">ViewManager</_this><container type="Folder">%s</container><type>%s</type><recursive>true</recursive></CreateContainerView>`
	soapRetrieve                = `<RetrieveProperties xmlns="urn:vim25"><_this type="PropertyCollector">propertyCollector</_this><specSet><propSet><type>%s</type>%s</propSet><objectSet><obj type="ContainerView">%s</obj><skip>true</skip><selectSet xsi:type="TraversalSpec"><name>traverseEntities</name><type>ContainerView</type><path>view</path><skip>false</skip></selectSet></objectSet></specSet></RetrieveProperties>`
	soapRetrieveDatastore       = `<RetrieveProperties xmlns="urn:vim25"><_this type="PropertyCollector">propertyCollector</_this><specSet><propSet><type>Datastore</type><all>true</all></propSet>%s</specSet></RetrieveProperties>`
	soapPerfProviderSummary     = `<QueryPerfProviderSummary xmlns="urn:vim25" xsi:type="QueryPerfProviderSummaryRequestType"><_this type="PerformanceManager">%s</_this><entity type="%s">%s</entity></QueryPerfProviderSummary>`
	soapQueryPerfCounters       = `<QueryPerfCounter xmlns="urn:vim25" xsi:type="QueryPerfCounterRequestType"><_this type="PerformanceManager">%s</_this>%s</QueryPerfCounter>`
	soapQueryPerf               = `<QueryPerf xmlns="urn:vim25" xsi:type="QueryPerfRequestType"><_this type="PerformanceManager">%s</_this><querySpec><entity type="%s">%s</entity><maxSample>1</maxSample><intervalId>%d</intervalId></querySpec></QueryPerf>`
)
