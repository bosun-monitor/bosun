package sched

import (
	"testing"

	"bosun.org/cmd/bosun/conf"
	"bosun.org/cmd/bosun/conf/rule"
	"bosun.org/models"
)

func TestActionNotificationGrouping(t *testing.T) {
	defer setup()()
	c, err := rule.NewConf("", conf.EnabledBackends{}, nil, `
		template t{
			subject = 2
			body = 2
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
	s, err := initSched(&conf.SystemConf{}, c)
	if err != nil {
		t.Fatal(err)
	}
	awarn := models.AlertKey("a{host=w}")
	acrit := models.AlertKey("a{host=c}")
	bwarn := models.AlertKey("b{host=w}")
	bcrit := models.AlertKey("b{host=c}")
	cA := models.AlertKey("c{host=a}")
	cB := models.AlertKey("c{host=b}")
	da := s.DataAccess.State()
	da.UpdateIncidentState(&models.IncidentState{AlertKey: awarn, Alert: awarn.Name(), Tags: awarn.Group().Tags(), WorstStatus: models.StWarning, Events: []models.Event{{Status: models.StWarning}}})
	da.UpdateIncidentState(&models.IncidentState{AlertKey: acrit, Alert: acrit.Name(), Tags: acrit.Group().Tags(), WorstStatus: models.StCritical, Events: []models.Event{{Status: models.StCritical}}})
	da.UpdateIncidentState(&models.IncidentState{AlertKey: bwarn, Alert: bwarn.Name(), Tags: bwarn.Group().Tags(), WorstStatus: models.StWarning, Events: []models.Event{{Status: models.StWarning}}})
	da.UpdateIncidentState(&models.IncidentState{AlertKey: bcrit, Alert: bcrit.Name(), Tags: bcrit.Group().Tags(), WorstStatus: models.StCritical, Events: []models.Event{{Status: models.StCritical}}})
	da.UpdateIncidentState(&models.IncidentState{AlertKey: cA, Alert: cA.Name(), Tags: cA.Group().Tags(), WorstStatus: models.StWarning, Events: []models.Event{{Status: models.StWarning}}})
	da.UpdateIncidentState(&models.IncidentState{AlertKey: cB, Alert: cB.Name(), Tags: cB.Group().Tags(), WorstStatus: models.StWarning, Events: []models.Event{{Status: models.StWarning}}})

	groups, err := s.groupActionNotifications(models.ActionClose, []models.AlertKey{awarn, acrit, bwarn, bcrit, cA, cB})
	if err != nil {
		t.Fatal(err)
	}
	expect := func(not string, aks ...models.AlertKey) {
		n := c.Notifications[not]
		key := notificationGroupKey{notification: n, template: c.Templates["t"]}
		actualAks, ok := groups[key]
		if !ok {
			t.Fatalf("Notification %s not present in groupings.", not)
		}
		if len(actualAks) != len(aks) {
			t.Fatalf("Count mismatch for grouping %s. %d != %d.", not, len(actualAks), len(aks))
		}
		for i, ak := range aks {
			if actualAks[i].AlertKey != ak {
				t.Fatalf("Alert key mismatch at index %d. %s != %s.", i, actualAks[i].AlertKey, ak)
			}
		}
	}
	expect("n1", awarn, cA, cB)
	expect("n2", acrit, bwarn, cA)
	expect("n3", bcrit, cB)
}
