package dbtest

import (
	"testing"
	"time"

	"bosun.org/models"
)

func TestSilence(t *testing.T) {
	sd := testData.Silence()

	silence := &models.Silence{
		Start: time.Now().Add(-48 * time.Hour),
		End:   time.Now().Add(5 * time.Hour),
		Alert: "Foo",
	}
	future := &models.Silence{
		Start: time.Now().Add(1 * time.Hour),
		End:   time.Now().Add(2 * time.Hour),
		Alert: "Foo",
	}
	past := &models.Silence{
		Start: time.Now().Add(-48 * time.Hour),
		End:   time.Now().Add(-5 * time.Hour),
		Alert: "Foo",
	}

	check(t, sd.AddSilence(silence))
	check(t, sd.AddSilence(past))
	check(t, sd.AddSilence(future))

	active, err := sd.GetActiveSilences()
	check(t, err)
	if len(active) != 1 {
		t.Fatalf("Expected only one active silence. Got %d.", len(active))
	}

}
