package database

import (
	"bosun.org/models"
	"encoding/json"
	"fmt"
	"github.com/garyburd/redigo/redis"
	"time"
)

/*

failingAlerts = set of currently failing alerts
alertsWithErrors = set of alerts with any errors
errorEvents = list of (alert) one per individual error event
error:{name} = list of json objects for coalesced error events (most recent first).

*/

type ErrorDataAccess interface {
	MarkAlertSuccess(name string) error
	MarkAlertFailure(name string, msg string) error
	GetFailingAlertCounts() (int, int, error)

	GetFailingAlerts() (map[string]bool, error)
	IsAlertFailing(name string) (bool, error)

	GetFullErrorHistory() (map[string][]*models.AlertError, error)
	ClearAlert(name string) error
	ClearAll() error
}

func (d *dataAccess) Errors() ErrorDataAccess {
	return d
}

const (
	failingAlerts    = "failingAlerts"
	errorEvents      = "errorEvents"
	alertsWithErrors = "alertsWithErrors"
)

func (d *dataAccess) MarkAlertSuccess(name string) error {
	conn := d.Get()
	defer conn.Close()
	_, err := conn.Do("SREM", failingAlerts, name)
	return err
}

func (d *dataAccess) MarkAlertFailure(name string, msg string) error {
	conn := d.Get()
	defer conn.Close()

	failing, err := d.IsAlertFailing(name)
	if err != nil {
		return err
	}

	if _, err := conn.Do("SADD", alertsWithErrors, name); err != nil {
		return err
	}
	if _, err := conn.Do("SADD", failingAlerts, name); err != nil {
		return err
	}
	var event *models.AlertError
	if failing {
		event, err = d.getLastErrorEvent(name)
		if err != nil {
			return err
		}
	}
	now := time.Now().UTC().Truncate(time.Second)
	if event == nil || event.Message != msg {
		event = &models.AlertError{
			FirstTime: now,
			LastTime:  now,
			Count:     1,
			Message:   msg,
		}
	} else {
		event.Count++
		event.LastTime = now
		// pop prior record
		_, err = conn.Do("LPOP", errorListKey(name))
		if err != nil {
			return err
		}
	}
	marshalled, err := json.Marshal(event)
	if err != nil {
		return err
	}
	_, err = conn.Do("LPUSH", errorListKey(name), marshalled)
	if err != nil {
		return err
	}
	_, err = conn.Do("LPUSH", errorEvents, name)
	return err
}

func (d *dataAccess) GetFailingAlertCounts() (int, int, error) {
	conn := d.Get()
	defer conn.Close()
	failing, err := redis.Int(conn.Do("SCARD", failingAlerts))
	if err != nil {
		return 0, 0, err
	}
	events, err := redis.Int(conn.Do("LLEN", errorEvents))
	if err != nil {
		return 0, 0, err
	}
	return failing, events, nil
}

func (d *dataAccess) GetFailingAlerts() (map[string]bool, error) {
	conn := d.Get()
	defer conn.Close()
	alerts, err := redis.Strings(conn.Do("SMEMBERS", failingAlerts))
	if err != nil {
		return nil, err
	}
	r := make(map[string]bool, len(alerts))
	for _, a := range alerts {
		r[a] = true
	}
	return r, nil
}
func (d *dataAccess) IsAlertFailing(name string) (bool, error) {
	conn := d.Get()
	defer conn.Close()
	return redis.Bool(conn.Do("SISMEMBER", failingAlerts, name))
}

func errorListKey(name string) string {
	return fmt.Sprintf("errors:%s", name)
}
func (d *dataAccess) getLastErrorEvent(name string) (*models.AlertError, error) {
	conn := d.Get()
	defer conn.Close()
	str, err := redis.Bytes(conn.Do("LINDEX", errorListKey(name), "0"))
	if err != nil {
		if err == redis.ErrNil {
			return nil, nil
		}
		return nil, err
	}
	ev := &models.AlertError{}
	if err = json.Unmarshal(str, ev); err != nil {
		return nil, err
	}
	return ev, nil
}

func (d *dataAccess) GetFullErrorHistory() (map[string][]*models.AlertError, error) {
	conn := d.Get()
	defer conn.Close()

	alerts, err := redis.Strings(conn.Do("SMEMBERS", alertsWithErrors))
	if err != nil {
		return nil, err
	}
	results := make(map[string][]*models.AlertError, len(alerts))
	for _, a := range alerts {
		rows, err := redis.Strings(conn.Do("LRANGE", errorListKey(a), 0, -1))
		if err != nil {
			return nil, err
		}
		list := make([]*models.AlertError, len(rows))
		for i, row := range rows {
			ae := &models.AlertError{}
			err = json.Unmarshal([]byte(row), ae)
			if err != nil {
				return nil, err
			}
			list[i] = ae
		}
		results[a] = list
	}
	return results, nil
}

func (d *dataAccess) ClearAlert(name string) error {
	conn := d.Get()
	defer conn.Close()

	_, err := conn.Do("SREM", alertsWithErrors, name)
	if err != nil {
		return err
	}
	_, err = conn.Do("SREM", failingAlerts, name)
	if err != nil {
		return err
	}
	_, err = conn.Do(d.LCLEAR(), errorListKey(name))
	if err != nil {
		return err
	}
	cmd, args := d.LMCLEAR(errorEvents, name)
	_, err = conn.Do(cmd, args...)
	if err != nil {
		return err
	}

	return nil
}

//Things could forseeably get a bit inconsistent if concurrent changes happen in just the wrong way.
//Clear all should do a more thourogh cleanup to fully reset things.
func (d *dataAccess) ClearAll() error {
	conn := d.Get()
	defer conn.Close()

	alerts, err := redis.Strings(conn.Do("SMEMBERS", alertsWithErrors))
	if err != nil {
		return err
	}
	for _, a := range alerts {
		if _, err := conn.Do(d.LCLEAR(), errorListKey(a)); err != nil {
			return err
		}
	}
	if _, err := conn.Do(d.SCLEAR(), alertsWithErrors); err != nil {
		return err
	}
	if _, err := conn.Do(d.SCLEAR(), failingAlerts); err != nil {
		return err
	}
	if _, err = conn.Do(d.LCLEAR(), errorEvents); err != nil {
		return err
	}

	return nil
}
