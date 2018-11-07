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
	periodSilence1 := &models.Silence{
		// 00:00:00 ~ 08:00:00
		Start: time.Now().Add(-48 * time.Hour),
		End:   time.Now().Add(5 * time.Hour),
		PeriodTimeIntStart: 0,
		PeriodTimeIntEnd: 80000,
		Alert: "Foo",
	}
	periodSilence2 := &models.Silence{
		// 23:00:00 ~ 08:30:00
		Start: time.Now().Add(-48 * time.Hour),
		End:   time.Now().Add(5 * time.Hour),
		PeriodTimeIntStart: 230000,
		PeriodTimeIntEnd: 83000,
		Alert: "Foo",
	}

	check(t, sd.AddSilence(silence))
	check(t, sd.AddSilence(past))
	check(t, sd.AddSilence(future))
	check(t, sd.AddSilence(periodSilence1))
	check(t, sd.AddSilence(periodSilence2))

	active, err := sd.GetActiveSilences()
	check(t, err)
	if len(active) != 1 {
		t.Fatalf("Expected only one active silence. Got %d.", len(active))
	}

}
