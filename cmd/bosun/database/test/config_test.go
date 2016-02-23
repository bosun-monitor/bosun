package dbtest

import "testing"

func TestConfigSave(t *testing.T) {
	cd := testData.Configs()

	hash, err := cd.SaveTempConfig("test123")
	check(t, err)

	recoverd, err := cd.GetTempConfig(hash)
	check(t, err)
	if recoverd != "test123" {
		t.Fatalf("Loaded config doesn't match: %s", recoverd)
	}
}
