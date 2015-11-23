package database

import (
	"encoding/json"
	"time"

	"bosun.org/_third_party/github.com/garyburd/redigo/redis"
	"bosun.org/collect"
	"bosun.org/models"
	"bosun.org/opentsdb"
)

/*

incidents: hash of {id} -> json of incident
maxIncidentId: counter. Increment to get next id.
incidentsByStart: sorted set by start date

*/

type IncidentDataAccess interface {
	GetIncident(id uint64) (*models.Incident, error)
	CreateIncident(ak models.AlertKey, start time.Time) (*models.Incident, error)
	UpdateIncident(id uint64, i *models.Incident) error

	GetIncidentsStartingInRange(start, end time.Time) ([]*models.Incident, error)

	// should only be used by initial import
	SetMaxId(id uint64) error
}

func (d *dataAccess) Incidents() IncidentDataAccess {
	return d
}

func (d *dataAccess) GetIncident(id uint64) (*models.Incident, error) {
	defer collect.StartTimer("redis", opentsdb.TagSet{"op": "GetIncident"})()
	conn := d.GetConnection()
	defer conn.Close()
	raw, err := redis.Bytes(conn.Do("HGET", "incidents", id))
	if err != nil {
		return nil, err
	}
	incident := &models.Incident{}
	if err = json.Unmarshal(raw, incident); err != nil {
		return nil, err
	}
	return incident, nil
}

func (d *dataAccess) CreateIncident(ak models.AlertKey, start time.Time) (*models.Incident, error) {
	defer collect.StartTimer("redis", opentsdb.TagSet{"op": "CreateIncident"})()
	conn := d.GetConnection()
	defer conn.Close()
	id, err := redis.Int64(conn.Do("INCR", "maxIncidentId"))
	if err != nil {
		return nil, err
	}
	incident := &models.Incident{
		Id:       uint64(id),
		Start:    start,
		AlertKey: ak,
	}
	err = saveIncident(incident.Id, incident, conn)
	if err != nil {
		return nil, err
	}
	return incident, nil
}

func saveIncident(id uint64, i *models.Incident, conn redis.Conn) error {
	raw, err := json.Marshal(i)
	if err != nil {
		return err
	}
	if _, err = conn.Do("HSET", "incidents", id, raw); err != nil {
		return err
	}
	if _, err = conn.Do("ZADD", "incidentsByStart", i.Start.UTC().Unix(), id); err != nil {
		return err
	}
	return nil
}

func (d *dataAccess) GetIncidentsStartingInRange(start, end time.Time) ([]*models.Incident, error) {
	defer collect.StartTimer("redis", opentsdb.TagSet{"op": "GetIncidentsStartingInRange"})()
	conn := d.GetConnection()
	defer conn.Close()

	ids, err := redis.Ints(conn.Do("ZRANGEBYSCORE", "incidentsByStart", start.UTC().Unix(), end.UTC().Unix()))
	if err != nil {
		return nil, err
	}
	args := make([]interface{}, len(ids)+1)
	args[0] = "incidents"
	for i := range ids {
		args[i+1] = ids[i]
	}
	jsons, err := redis.Strings(conn.Do("HMGET", args...))
	if err != nil {
		return nil, err
	}
	incidents := make([]*models.Incident, len(jsons))
	for i := range jsons {
		inc := &models.Incident{}
		if err = json.Unmarshal([]byte(jsons[i]), inc); err != nil {
			return nil, err
		}
		incidents[i] = inc
	}
	return incidents, nil
}

func (d *dataAccess) UpdateIncident(id uint64, i *models.Incident) error {
	defer collect.StartTimer("redis", opentsdb.TagSet{"op": "UpdateIncident"})()
	conn := d.GetConnection()
	defer conn.Close()
	return saveIncident(id, i, conn)
}

func (d *dataAccess) SetMaxId(id uint64) error {
	defer collect.StartTimer("redis", opentsdb.TagSet{"op": "SetMaxId"})()
	conn := d.GetConnection()
	defer conn.Close()

	_, err := conn.Do("SET", "maxIncidentId", id)
	return err
}
