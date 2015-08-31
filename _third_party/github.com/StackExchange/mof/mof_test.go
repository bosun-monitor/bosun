package mof

import (
	"encoding/json"
	"io/ioutil"
	"testing"
)

func TestUnmarshal(t *testing.T) {
	tests := []struct {
		filename string
		typ      interface{}
		expected string
	}{
		// ASCII
		{"ascii.mof", MSFT_DSCConfigurationStatus{}, asciiMOFExpected},

		// UTF-16, little-endian
		{"utf16-le.mof", MSFT_DSCConfigurationStatus{}, utf16leMOFExpected},
	}
	for _, test := range tests {
		b, err := ioutil.ReadFile(test.filename)
		if err != nil {
			t.Fatal(err)
		}
		var v interface{}
		if err := Unmarshal(b, &v); err != nil {
			t.Fatal(err)
		}
		s := test.typ
		if err := Unmarshal(b, &s); err != nil {
			t.Fatal(err)
		}
		vb, _ := json.MarshalIndent(&v, "", "  ")
		sb, _ := json.MarshalIndent(&s, "", "  ")
		if string(vb) != test.expected {
			t.Errorf("%v: unexpected interface value", test.filename)
		}
		if string(sb) != test.expected {
			t.Errorf("%v: unexpected struct value", test.filename)
		}
	}
}

type MSFT_DSCConfigurationStatus struct {
	MSFT_DSCConfigurationStatus []struct {
		DurationInSeconds int
		Error             string
		HostName          string
		IPV4Addresses     []string
		IPV6Addresses     []string
		JobID             string
		LCMVersion        string
		Locale            string
		MACAddresses      []string
		MetaConfiguration struct {
			AllowModuleOverwrite           bool
			ConfigurationDownloadManagers  []interface{}
			ConfigurationMode              string
			ConfigurationModeFrequencyMins int
			DebugMode                      []string
			LCMCompatibleVersions          []string
			LCMState                       string
			LCMVersion                     string
			RebootNodeIfNeeded             bool
			RefreshFrequencyMins           int
			RefreshMode                    string
			ReportManagers                 []interface{}
			ResourceModuleManagers         []interface{}
			StatusRetentionTimeInDays      int
		}
		Mode            string
		RebootRequested bool
		StartDate       string
		Status          string
		Type            string
	}
	ReturnValue int
}

const (
	asciiMOFExpected = `{
  "MSFT_DSCConfigurationStatus": [
    {
      "DurationInSeconds": 0,
      "Error": "Current configuration does not exist. Execute Start-DscConfiguration command with -Path parameter to specify a configuration file and create a current configuration first.",
      "HostName": "HOSTNAME-VM",
      "IPV4Addresses": [
        "1.2.3.4",
        "127.0.0.1"
      ],
      "IPV6Addresses": [
        "fe80::9aac:52eb:f6ca:a32b%3",
        "::2000:0:0:0",
        "::1",
        "::2000:0:0:0"
      ],
      "JobID": "{1D75864B-1451-11E5-8276-08002721727D}",
      "LCMVersion": "2.0",
      "Locale": "en-US",
      "MACAddresses": [
        "28-09-35-11-42-8A",
        "00-00-00-00-00-00-00-E0"
      ],
      "MetaConfiguration": {
        "AllowModuleOverwrite": false,
        "ConfigurationDownloadManagers": [],
        "ConfigurationMode": "ApplyAndMonitor",
        "ConfigurationModeFrequencyMins": 15,
        "DebugMode": [
          "NONE"
        ],
        "LCMCompatibleVersions": [
          "1.0",
          "2.0"
        ],
        "LCMState": "Idle",
        "LCMVersion": "2.0",
        "RebootNodeIfNeeded": false,
        "RefreshFrequencyMins": 30,
        "RefreshMode": "PUSH",
        "ReportManagers": [],
        "ResourceModuleManagers": [],
        "StatusRetentionTimeInDays": 10
      },
      "Mode": "PUSH",
      "RebootRequested": false,
      "StartDate": "2015/06/16 13:57:13",
      "Status": "Failure",
      "Type": "ReadOnly"
    }
  ],
  "ReturnValue": 0
}`

	utf16leMOFExpected = `{
  "MSFT_DSCConfigurationStatus": [
    {
      "DurationInSeconds": 8127,
      "HostName": "GBRAY02",
      "IPV4Addresses": [
        "192.168.0.7",
        "172.28.19.1",
        "127.0.0.1"
      ],
      "IPV6Addresses": [
        "fe80::ec13:deaa:d70c:615c%3",
        "::2000:0:0:0",
        "fe80::51c1:6e25:a2b1:6053%26",
        "::2000:0:0:0",
        "::1",
        "::2000:0:0:0"
      ],
      "JobID": "{79D1FB3D-0F06-11E5-8288-D48564B35523}",
      "LCMVersion": "2.0",
      "Locale": "en-US",
      "MACAddresses": [
        "D4-85-64-B3-55-23",
        "08-00-27-00-4C-02",
        "00-00-00-00-00-00-00-E0",
        "00-00-00-00-00-00-00-E0"
      ],
      "MetaConfiguration": {
        "AllowModuleOverwrite": false,
        "ConfigurationDownloadManagers": [],
        "ConfigurationMode": "ApplyAndMonitor",
        "ConfigurationModeFrequencyMins": 15,
        "DebugMode": [
          "NONE"
        ],
        "LCMCompatibleVersions": [
          "1.0",
          "2.0"
        ],
        "LCMState": "Ready",
        "LCMVersion": "2.0",
        "RebootNodeIfNeeded": false,
        "RefreshFrequencyMins": 30,
        "RefreshMode": "PUSH",
        "ReportManagers": [],
        "ResourceModuleManagers": [],
        "StatusRetentionTimeInDays": 10
      },
      "MetaData": "Author: gbray; Name: SEConfig_Simple; Version: 2.0.0; GenerationDate: 06/08/2015 16:42:06; GenerationHost: GBRAY02;",
      "Mode": "PUSH",
      "NumberOfResources": 1,
      "RebootRequested": false,
      "StartDate": "2015/06/09 18:20:20",
      "Status": "Success",
      "Type": "Reboot"
    }
  ],
  "ReturnValue": 0
}`
)
