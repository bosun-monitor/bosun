package tsdbs

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"bosun.org/models"
)

// AzureResources is a slice of AzureResource
type AzureResources struct {
	Resources []AzureResource
	Prefix    string
}

// AzureResource is a container for Azure resource information that Bosun can interact with.
type AzureResource struct {
	Name          string
	Type          string
	ResourceGroup string
	Tags          map[string]string
	ID            string
}

func (a AzureResources) Type() models.FuncType { return models.TypeAzureResourceList }
func (a AzureResources) Value() interface{}    { return a }

// AzureApplicationInsightsApp in collection of properties for each Azure Application Insights Resource
type AzureApplicationInsightsApp struct {
	ApplicationName string
	AppId           string
	Tags            map[string]string
}

// Get Returns an AzureResource from AzureResources based on the resource type, group, and name
// If no matching resource is found, an AzureResource object will be returned but found will be
// false.
func (resources AzureResources) Get(rType, rsg, name string) (az AzureResource, found bool) {
	for _, res := range resources.Resources {
		if res.Type == rType && res.ResourceGroup == rsg && res.Name == name {
			return res, true
		}
	}
	return
}

// Ask makes an AzureResource a github.com/kylebrandt/boolq Asker, which allows it
// to take boolean expressions to create true/false conditions for filtering
func (ar AzureResource) Ask(filter string) (bool, error) {
	sp := strings.SplitN(filter, ":", 2)
	if len(sp) != 2 {
		return false, fmt.Errorf("bad filter, filter must be in k:v format, got %v", filter)
	}
	key := strings.ToLower(sp[0]) // Make key case insensitive
	value := sp[1]
	switch key {
	case AzureTagName:
		re, err := regexp.Compile(value)
		if err != nil {
			return false, err
		}
		if re.MatchString(ar.Name) {
			return true, nil
		}
	case AzureTagRSG:
		re, err := regexp.Compile(value)
		if err != nil {
			return false, err
		}
		if re.MatchString(ar.ResourceGroup) {
			return true, nil
		}
	default:
		// Does not support tags that have a tag key of rsg, resourcegroup, or name. If it is a problem at some point
		// we can do something like "\name" to mean the tag "name" if such thing is even allowed
		if tagV, ok := ar.Tags[key]; ok {
			re, err := regexp.Compile(value)
			if err != nil {
				return false, err
			}
			if re.MatchString(tagV) {
				return true, nil
			}
		}

	}
	return false, nil
}

const (
	// constants for Azure tag keys
	AzureTagName = "name"
	AzureTagRSG  = "rsg"
)

// AzureApplicationInsightsApps is a container for a list of AzureApplicationInsightsApp objects
// It is a bosun type since it passed to Azure Insights query functions
type AzureApplicationInsightsApps struct {
	Applications []AzureApplicationInsightsApp
	Prefix       string
}

func (a AzureApplicationInsightsApps) Type() models.FuncType { return models.TypeAzureAIApps }
func (a AzureApplicationInsightsApps) Value() interface{}    { return a }

// Ask makes an AzureApplicationInsightsApp a github.com/kylebrandt/boolq Asker, which allows it to
// to take boolean expressions to create true/false conditions for filtering
func (app AzureApplicationInsightsApp) Ask(filter string) (bool, error) {
	sp := strings.SplitN(filter, ":", 2)
	if len(sp) != 2 {
		return false, fmt.Errorf("bad filter, filter must be in k:v format, got %v", filter)
	}
	key := strings.ToLower(sp[0]) // Make key case insensitive
	value := sp[1]
	switch key {
	case AzureTagName:
		re, err := regexp.Compile(value)
		if err != nil {
			return false, err
		}
		if re.MatchString(app.ApplicationName) {
			return true, nil
		}
	default:
		if tagV, ok := app.Tags[key]; ok {
			re, err := regexp.Compile(value)
			if err != nil {
				return false, err
			}
			if re.MatchString(tagV) {
				return true, nil
			}
		}

	}
	return false, nil
}

type ESQuery struct {
	Query func(ver string) interface{}
}

func (e ESQuery) Type() models.FuncType { return models.TypeESQuery }
func (e ESQuery) Value() interface{}    { return e }
func (e ESQuery) MarshalJSON() ([]byte, error) {
	// source, err := e.Query(esV2).Source()
	// if err != nil {
	// 	return nil, err
	// }
	// return json.Marshal(source)
	return json.Marshal("ESQuery")
}

type ESIndexer struct {
	TimeField string
	Generate  func(startDuration, endDuration *time.Time) []string
}

func (e ESIndexer) Type() models.FuncType { return models.TypeESIndexer }
func (e ESIndexer) Value() interface{}    { return e }
func (e ESIndexer) MarshalJSON() ([]byte, error) {
	return json.Marshal("ESGenerator")
}
