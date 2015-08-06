package pstree

import (
	"os"
	"testing"
	"time"

	"bosun.org/_third_party/github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	timeStart := time.Unix(0, 0)
	time1 := timeStart.Add(time.Duration(1))
	os.Chtimes("test/proc/1/status", time1, time1)
	time20 := timeStart.Add(time.Duration(20))
	os.Chtimes("test/proc/20/status", time20, time20)
	time21 := timeStart.Add(time.Duration(21))
	os.Chtimes("test/proc/21/status", time21, time21)
	time30 := timeStart.Add(time.Duration(30))
	os.Chtimes("test/proc/30/status", time30, time30)
	os.Exit(m.Run())
}

func TestTreeMapID(t *testing.T) {
	psTree, err := NewFromDir("test/proc")
	assert.Nil(t, err)
	assert.NotNil(t, psTree)
	expectedSubTreeMap := map[string]int{
		"1":  1,
		"20": 2,
		"21": 3,
		"30": 4,
	}
	subTreeMapID := psTree.SubTreeMapID(1)
	assert.Equal(t, expectedSubTreeMap, subTreeMapID)
}
