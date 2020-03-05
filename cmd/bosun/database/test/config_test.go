package dbtest

import (
	"bosun.org/host"
	"bosun.org/util"
	"testing"
)

func TestConfigSave(t *testing.T) {
	hm, err := host.NewManager(false)
	if err != nil {
		t.Error(err)
	}
	util.SetHostManager(hm)

	cd := testData.Configs()

	hash, err := cd.SaveTempConfig("test123")
	check(t, err)

	recoverd, err := cd.GetTempConfig(hash)
	check(t, err)
	if recoverd != "test123" {
		t.Fatalf("Loaded config doesn't match: %s", recoverd)
	}
}
