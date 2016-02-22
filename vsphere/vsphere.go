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
)

// Vsphere holds connection state for a vSphere session.
type Vsphere struct {
	url     *url.URL
	header  http.Header
	cookies http.CookieJar
	client  *http.Client
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
	uuid := struct {
		UUID string `xml:"Body>RetrieveServiceContentResponse>returnval>about>instanceUuid"`
	}{}
	if err := v.call(soapConnect, &uuid); err != nil {
		return nil, err
	}
	if uuid.UUID == "" {
		return nil, fmt.Errorf("vsphere: no UUID during connect")
	}
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
)
