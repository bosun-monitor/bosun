package dbtest

import (
	"testing"
	"time"

	"bosun.org/models"
)

func TestNotifications_RoundTrip(t *testing.T) {

	nd := testData.Notifications()
	notTime := time.Now().UTC().Add(-10 * time.Hour).Truncate(time.Second)
	future := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	oneMin := time.Now().UTC().Add(time.Minute).Truncate(time.Second)

	// with nothing pending, next time should be an hour from now
	next, err := nd.GetNextNotificationTime()
	check(t, err)
	if next != future {
		t.Fatalf("wrong next time. %s != %s", next, future)
	}

	// add notifications
	err = nd.InsertNotification(models.AlertKey("notak{foo=a}"), "chat", notTime)
	check(t, err)
	err = nd.InsertNotification(models.AlertKey("notak{foo=b}"), "chat", oneMin)
	check(t, err)
	err = nd.InsertNotification(models.AlertKey("notak{foo=c}"), "chat", future)
	check(t, err)

	// next time should be correct
	next, err = nd.GetNextNotificationTime()
	check(t, err)
	if next != notTime {
		t.Fatalf("wrong next time. %s != %s", next, notTime)
	}

	// make sure only one due
	due, err := nd.GetDueNotifications()
	check(t, err)
	if len(due) != 1 {
		t.Fatalf("Wrong number of due notifications. %d != %d", len(due), 1)
	}

	// next time should still be correct
	next, err = nd.GetNextNotificationTime()
	check(t, err)
	if next != notTime {
		t.Fatalf("wrong next time. %s != %s", next, notTime)
	}

	check(t, nd.ClearNotificationsBefore(notTime))
	// next time should be 1 minute
	next, err = nd.GetNextNotificationTime()
	check(t, err)
	if next != oneMin {
		t.Fatalf("wrong next time. %s != %s", next, oneMin)
	}

	check(t, nd.ClearNotifications(models.AlertKey("notak{foo=b}")))
	// next time should be 1 hour
	next, err = nd.GetNextNotificationTime()
	check(t, err)
	if next != future {
		t.Fatalf("wrong next time. %s != %s", next, future)
	}
}
