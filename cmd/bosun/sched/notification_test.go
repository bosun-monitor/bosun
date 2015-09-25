package sched

import (
	"bytes"
	"strings"
	"testing"

	"bosun.org/cmd/bosun/conf"
	"bosun.org/cmd/bosun/expr"
	"bosun.org/opentsdb"
)

func TestActionNotificationTemplates(t *testing.T) {
	c, err := conf.New("", `hostname = abc`)
	c.StateFile = ""
	if err != nil {
		t.Fatal(err)
	}
	s, _ := initSched(c)
	data := &actionNotificationContext{}
	data.ActionType = ActionAcknowledge
	data.Message = "Bad things happened"
	data.User = "Batman"
	data.States = []*State{
		{
			History: []Event{
				{
					Status:     StCritical,
					IncidentId: 224,
				},
			},
			Alert:   "xyz",
			Subject: "Critical!!",
		},
	}
	data.schedule = s
	buf := &bytes.Buffer{}
	err = actionNotificationBodyTemplate.Execute(buf, data)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "http://abc/incident?id=224") {
		t.Fatal("Expected link to incident in body")
	}
	buf = &bytes.Buffer{}
	err = actionNotificationSubjectTemplate.Execute(buf, data)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "Batman Acknowledged") {
		t.Fatal("Expected name and actionType in subject")
	}
}

func TestActionNotificationGrouping(t *testing.T) {
	c, err := conf.New("", `
		template t{
			subject = 2
		}
		notification n1 {
			print = true
		}
		notification n2{
			print = true
		}
		notification n3{
			print = true
			runOnActions = true
		}
		notification n4{
			print = true
			runOnActions = false
		}
		alert a {
			template = t
			warnNotification = n1
			critNotification = n2
			warnNotification = n4
			crit = 1
			warn = 1
		}
		alert b{
			template = t
			warnNotification = n2
			critNotification = n3
			crit = 1
			warn = 1
		}
		lookup byHost{
			entry host=a{
				main_contact = n2
			}
			entry host=b{
				main_contact = n3
			}
		}
		alert c{
			template = t
			warnNotification = n1
			warnNotification = lookup("byHost", "main_contact")
			warn = 1
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	s, err := initSched(c)
	if err != nil {
		t.Fatal(err)
	}
	awarn := expr.AlertKey("a{host=w}")
	acrit := expr.AlertKey("a{host=c}")
	bwarn := expr.AlertKey("b{host=w}")
	bcrit := expr.AlertKey("b{host=c}")
	cA := expr.AlertKey("c{host=a}")
	cB := expr.AlertKey("c{host=b}")
	s.status[awarn] = &State{Alert: "a", Group: opentsdb.TagSet{"host": "w"}, History: []Event{{Status: StWarning}}}
	s.status[acrit] = &State{Alert: "a", Group: opentsdb.TagSet{"host": "c"}, History: []Event{{Status: StCritical}}}
	s.status[bwarn] = &State{Alert: "b", Group: opentsdb.TagSet{"host": "w"}, History: []Event{{Status: StWarning}}}
	s.status[bcrit] = &State{Alert: "b", Group: opentsdb.TagSet{"host": "c"}, History: []Event{{Status: StCritical}}}
	s.status[cA] = &State{Alert: "c", Group: opentsdb.TagSet{"host": "a"}, History: []Event{{Status: StWarning}}}
	s.status[cB] = &State{Alert: "c", Group: opentsdb.TagSet{"host": "b"}, History: []Event{{Status: StWarning}}}

	groups := s.groupActionNotifications([]expr.AlertKey{awarn, acrit, bwarn, bcrit, cA, cB})
	expect := func(not string, aks ...expr.AlertKey) {
		n := c.Notifications[not]
		actualAks, ok := groups[n]
		if !ok {
			t.Fatalf("Notification %s not present in groupings.", not)
		}
		if len(actualAks) != len(aks) {
			t.Fatalf("Count mismatch for grouping %s. %d != %d.", not, len(actualAks), len(aks))
		}
		for i, ak := range aks {
			if actualAks[i].AlertKey() != ak {
				t.Fatalf("Alert key mismatch at index %d. %s != %s.", i, actualAks[i].AlertKey(), ak)
			}
		}
	}
	expect("n1", awarn, cA, cB)
	expect("n2", acrit, bwarn, cA)
	expect("n3", bcrit, cB)
}
