package dbtest

import (
	"testing"
)

func TestErrors_RoundTrip(t *testing.T) {
	ed := testData.Errors()
	alert := "abcdefg"

	// make sure we can mark success and error
	check(t, ed.MarkAlertSuccess(alert))
	failing, err := ed.IsAlertFailing(alert)
	check(t, err)
	if failing {
		t.Fatal("Alert should not be failing")
	}
	check(t, ed.MarkAlertFailure(alert, "Something bad happened"))
	failing, err = ed.IsAlertFailing(alert)
	check(t, err)
	if !failing {
		t.Fatal("Alert should  be failing")
	}

	// generate a sequence of errors. We should have groups of 2/1/1 (oldest to newest)
	check(t, ed.MarkAlertFailure(alert, "Something bad happened"))
	check(t, ed.MarkAlertSuccess(alert))
	check(t, ed.MarkAlertFailure(alert, "Something bad happened"))
	check(t, ed.MarkAlertFailure(alert, "Something different bad happened"))

	failingCount, events, err := ed.GetFailingAlertCounts()
	check(t, err)
	if failingCount != 1 {
		t.Fatalf("Expected 1 failing alert. Got %d", failingCount)
	}
	if events != 4 {
		t.Fatalf("Expected 1 error events. Got %d", events)
	}

	fullData, err := ed.GetFullErrorHistory()
	check(t, err)
	if len(fullData) != 1 {
		t.Fatalf("Expected data for 1 alert. See %d", len(fullData))
	}
	ad := fullData[alert]
	if len(ad) != 3 {
		t.Fatalf("Expected data for alert to have 3 entries. See %d", len(ad))
	}
	if ad[0].Count != 1 {
		t.Fatalf("Expected first entry to have length 1. Found %d.", ad[0].Count)
	}
	if ad[1].Count != 1 {
		t.Fatalf("Expected second entry to have length 1. Found %d.", ad[1].Count)
	}
	if ad[2].Count != 2 {
		t.Fatalf("Expected third entry to have length 2. Found %d.", ad[2].Count)
	}

	check(t, ed.ClearAlert(alert))
	failingCount, events, err = ed.GetFailingAlertCounts()
	check(t, err)
	if failingCount != 0 {
		t.Fatalf("Expected 0 failing alert. Got %d", failingCount)
	}
	if events != 0 {
		t.Fatalf("Expected 0 error events. Got %d", events)
	}

	check(t, ed.MarkAlertFailure(alert, "Something bad happened"))
	check(t, ed.ClearAll())
	failingCount, events, err = ed.GetFailingAlertCounts()
	check(t, err)
	if failingCount != 0 {
		t.Fatalf("Expected 0 failing alert. Got %d", failingCount)
	}
	if events != 0 {
		t.Fatalf("Expected 0 error events. Got %d", events)
	}
}
