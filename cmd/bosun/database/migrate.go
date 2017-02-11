package database

import (
	"encoding/json"
	"time"

	"bosun.org/models"
	"bosun.org/slog"
	"github.com/beego/redigo/redis"
)

// Version 0 is the schema that was never verisoned
// Version 1 migrates rendered templates from

var schemaKey = "schemaVersion"

type Migration struct {
	UID     string
	Task    func(d *dataAccess) error
	Version int64
}

var tasks = []Migration{
	Migration{
		UID:     "Migrate Rendered Templates",
		Task:    migrateRenderedTemplates,
		Version: 1,
	},
}

type oldIncidentState struct {
	Id       int64
	Start    time.Time
	End      *time.Time
	AlertKey models.AlertKey
	Alert    string // helper data since AlertKeys don't serialize to JSON well
	Tags     string // string representation of Group

	*models.Result

	// Most recent last.
	Events  []models.Event  `json:",omitempty"`
	Actions []models.Action `json:",omitempty"`

	Subject      string
	Body         string
	EmailBody    []byte
	EmailSubject []byte
	Attachments  []*models.Attachment

	NeedAck bool
	Open    bool

	Unevaluated bool

	CurrentStatus models.Status
	WorstStatus   models.Status

	LastAbnormalStatus models.Status
	LastAbnormalTime   int64
}

func migrateRenderedTemplates(d *dataAccess) error {
	slog.Infoln("Running rendered template migration")

	ids, err := d.getAllIncidentIds()
	if err != nil {
		return err
	}

	conn := d.Get()
	defer conn.Close()

	for _, id := range ids {
		b, err := redis.Bytes(conn.Do("GET", incidentStateKey(id)))
		if err != nil {
			return slog.Wrap(err)
		}
		oldState := &oldIncidentState{}
		if err := json.Unmarshal(b, oldState); err != nil {
			slog.Wrap(err)
		}
		rt := &models.RenderedTemplates{
			Body:         oldState.Body,
			EmailBody:    oldState.EmailBody,
			EmailSubject: oldState.EmailSubject,
			Attachments:  oldState.Attachments,
		}
		if err := d.State().SetRenderedTemplates(oldState.Id, rt); err != nil {
			return slog.Wrap(err)
		}

		newState := &models.IncidentState{
			Id:       oldState.Id,
			Start:    oldState.Start,
			End:      oldState.End,
			AlertKey: oldState.AlertKey,
			Alert:    oldState.Alert,
			Tags:     oldState.Tags,
			Result:   oldState.Result,

			Events:             oldState.Events,
			Actions:            oldState.Actions,
			Subject:            oldState.Subject,
			NeedAck:            oldState.NeedAck,
			Open:               oldState.Open,
			Unevaluated:        oldState.Unevaluated,
			CurrentStatus:      oldState.CurrentStatus,
			WorstStatus:        oldState.WorstStatus,
			LastAbnormalStatus: oldState.LastAbnormalStatus,
			LastAbnormalTime:   oldState.LastAbnormalTime,
		}
		_ = newState
	}

	return nil
}

func (d *dataAccess) Migrate() error {
	slog.Infoln("checking migrations")
	conn := d.Get()
	defer conn.Close()

	// Since we didn't record a schema version from the start
	// we have to do some assumptions to see if this is a new
	// database, or if was a database before we started recording
	// a schema version number

	version, err := redis.Int64(conn.Do("GET", schemaKey))
	if err != nil {
		if err == redis.ErrNil {
			slog.Infoln("schema version not found in db")
			if _, err := redis.Bool(conn.Do("Get", "allIncidents")); err == redis.ErrNil {
				slog.Infoln("assuming new installation because allIncidents key not found")
				slog.Infoln("writing current schema version")
				version = SchemaVersion
				return nil
			}
		} else {
			return slog.Wrap(err)
		}
	}

	for _, task := range tasks {
		if task.Version > version {
			// Check if migration has been run if not that run
			err := task.Task(d)
			if err != nil {
				// Mark Migration as Complete
			}
		}
	}
	return nil
}
