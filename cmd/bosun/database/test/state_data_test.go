package dbtest

import (
	"bosun.org/cmd/bosun/database"
	"bosun.org/models"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestNoCleanupIfDurationZero(t *testing.T) {
	doneNotify := make(chan int64, 1)
	func() {
		testData.State().StartTTLEnforcerLoop(time.Duration(0), time.Duration(0))
		doneNotify <- 0
	}()
	select {
	case res := <-doneNotify:
		assert.EqualValues(t, 0, res)
	case <-time.After(10 * time.Second):
		assert.Fail(t, "timeout waiting for ttl enforcer loop to return")
	}
}

func TestCleanupOnlyRemovesIncidentsBeyondRetention(t *testing.T) {
	incidentBeyondRetention := buildIncident(1, time.Now().Add(-time.Hour*2))
	incidentWithinRetention := buildIncident(2, time.Now().Add(-time.Minute*2))

	testData.State().UpdateIncidentState(&incidentBeyondRetention)
	testData.State().UpdateIncidentState(&incidentWithinRetention)
	testData.State().SetRenderedTemplates(incidentBeyondRetention.Id, &models.RenderedTemplates{Subject: "NotNil"})
	testData.State().SetRenderedTemplates(incidentWithinRetention.Id, &models.RenderedTemplates{Subject: "NotNil"})

	assertIncidentExists(t, incidentBeyondRetention)
	assertRenderedTemplateExists(t, incidentBeyondRetention)

	config := database.NewRetentionConfig(time.Duration(time.Hour*1), time.Duration(time.Hour*1))
	testData.State().CleanupOldIncidents(config)

	assertIncidentRemoved(t, incidentBeyondRetention)
	assertIncidentExists(t, incidentWithinRetention)

	assertRenderedTemplateRemoved(t, incidentBeyondRetention)
	assertRenderedTemplateExists(t, incidentWithinRetention)
}

func TestCleanupRenderedTemplatesWithoutIncidentCleanup(t *testing.T) {
	incidentBeyondRetention := buildIncident(1, time.Now().Add(-time.Hour*2))

	testData.State().UpdateIncidentState(&incidentBeyondRetention)
	testData.State().SetRenderedTemplates(incidentBeyondRetention.Id, &models.RenderedTemplates{Subject: "NotNil"})

	assertIncidentExists(t, incidentBeyondRetention)
	assertRenderedTemplateExists(t, incidentBeyondRetention)

	config := database.NewRetentionConfig(time.Duration(time.Hour*1), time.Duration(0))
	testData.State().CleanupOldIncidents(config)

	assertIncidentExists(t, incidentBeyondRetention)
	assertRenderedTemplateRemoved(t, incidentBeyondRetention)
}

func TestCleanupIncidentCleanupRemovesReferences(t *testing.T) {
	incidentBeyondRetention := buildIncident(0, time.Now().Add(-time.Hour*2))

	testData.State().UpdateIncidentState(&incidentBeyondRetention)
	testData.State().SetRenderedTemplates(incidentBeyondRetention.Id, &models.RenderedTemplates{Subject: "NotNil"})

	incidents, _ := testData.State().GetAllIncidentsByAlertKey(models.AlertKey("TestAlertKey"))
	assert.NotEmpty(t, incidents)
	assertIncidentExists(t, incidentBeyondRetention)
	assertRenderedTemplateExists(t, incidentBeyondRetention)

	config := database.NewRetentionConfig(time.Duration(0), time.Duration(time.Hour*1))
	testData.State().CleanupOldIncidents(config)

	incidents, _ = testData.State().GetAllIncidentsByAlertKey(models.AlertKey("TestAlertKey"))
	assert.Empty(t, incidents)
	assertIncidentRemoved(t, incidentBeyondRetention)
	assertRenderedTemplateRemoved(t, incidentBeyondRetention)
}

func assertIncidentRemoved(t *testing.T, incident models.IncidentState) {
	state, _ := testData.State().GetIncidentState(incident.Id)
	assert.Nil(t, state)
}

func assertIncidentExists(t *testing.T, incident models.IncidentState) {
	state, _ := testData.State().GetIncidentState(incident.Id)
	assert.NotNil(t, state)
}

func assertRenderedTemplateExists(t *testing.T, incident models.IncidentState) {
	template, _ := testData.State().GetRenderedTemplates(incident.Id)
	assert.NotEmpty(t, template.Subject)
}

func assertRenderedTemplateRemoved(t *testing.T, incident models.IncidentState) {
	template, _ := testData.State().GetRenderedTemplates(incident.Id)
	assert.Empty(t, template.Subject)
}

func buildIncident(id int64, end time.Time) models.IncidentState {
	return models.IncidentState{
		Id:                 id,
		Start:              end.Add(-time.Hour),
		End:                &end,
		AlertKey:           "TestAlertKey",
		Alert:              "",
		Tags:               "",
		Result:             nil,
		Events:             nil,
		Actions:            nil,
		Subject:            "",
		NeedAck:            false,
		Open:               false,
		Unevaluated:        false,
		CurrentStatus:      0,
		WorstStatus:        0,
		LastAbnormalStatus: 0,
		LastAbnormalTime:   models.Epoch{},
		Notifications:      nil,
	}
}
