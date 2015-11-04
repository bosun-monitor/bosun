package dbtest

import (
	"testing"
	"time"
)

func TestIncidents_RoundTrip(t *testing.T) {
	inc := testData.Incidents()

	i, err := inc.CreateIncident("foo{host=3}", time.Now().UTC())
	check(t, err)

	i2, err := inc.CreateIncident("foo{host=3}", time.Now().UTC())
	check(t, err)

	if i.Id >= i2.Id {
		t.Fatal("Expect ids to be ascending")
	}

	readBack, err := inc.GetIncident(i.Id)
	check(t, err)
	if readBack.AlertKey != i.AlertKey {
		t.Fatal("Alert key's don't match")
	}

	tm := time.Now().UTC().Add(42 * time.Hour)
	i.End = &tm
	check(t, inc.UpdateIncident(i.Id, i))

	readBack, err = inc.GetIncident(i.Id)
	check(t, err)
	if *readBack.End != tm {
		t.Fatal("End times don't match")
	}
}

func TestIncidentSearch(t *testing.T) {
	inc := testData.Incidents()

	startTime := time.Now().Add(-5000 * time.Hour).UTC()

	i, err := inc.CreateIncident("foo2{host=3}", startTime)
	check(t, err)

	i2, err := inc.CreateIncident("foo2{host=4}", startTime)
	check(t, err)

	_, err = inc.CreateIncident("foo2{host=4}", startTime.Add(500*time.Hour))
	check(t, err)
	_, err = inc.CreateIncident("foo2{host=4}", startTime.Add(-500*time.Hour))
	check(t, err)
	i3, err := inc.CreateIncident("BAR{host=4}", startTime)
	check(t, err)

	results, err := inc.GetIncidentsStartingInRange(startTime.Add(-1*time.Hour), startTime.Add(time.Hour))
	check(t, err)

	if len(results) != 3 {
		t.Fatal("Wrong number of results")
	}
	if results[0].Id != i.Id || results[1].Id != i2.Id || results[2].Id != i3.Id {
		t.Fatal("Ids don't match", results, i.Id, i2.Id, i3.Id)
	}
}
