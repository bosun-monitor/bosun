package database

import (
	"fmt"
	"strings"
	"time"

	"github.com/garyburd/redigo/redis"

	"bosun.org/models"
	"bosun.org/slog"
)

/*

pendingNotifications: ZSET timestamp ak:notification

notsByAlert:alert SET of notifications possible per alert. used to clear alerts by alert key

*/

const (
	pendingNotificationsKey = "pendingNotifications"
)

func notsByAlertKeyKey(ak models.AlertKey) string {
	return fmt.Sprintf("notsByAlert:%s", ak.Name())
}

type NotificationDataAccess interface {
	InsertNotification(ak models.AlertKey, notification string, dueAt time.Time) error

	//Get notifications that are currently due or past due. Does not delete.
	GetDueNotifications() (map[models.AlertKey]map[string]time.Time, error)

	//Clear all notifications due on or before a given timestamp. Intended is to use the max returned from GetDueNotifications once you have processed them.
	ClearNotificationsBefore(time.Time) error

	ClearNotifications(ak models.AlertKey) error

	GetNextNotificationTime() (time.Time, error)
}

func (d *dataAccess) Notifications() NotificationDataAccess {
	return d
}

func (d *dataAccess) InsertNotification(ak models.AlertKey, notification string, dueAt time.Time) error {
	conn := d.Get()
	defer conn.Close()

	_, err := conn.Do("ZADD", pendingNotificationsKey, dueAt.UTC().Unix(), fmt.Sprintf("%s:%s", ak, notification))
	if err != nil {
		return slog.Wrap(err)
	}
	_, err = conn.Do("SADD", notsByAlertKeyKey(ak), notification)
	return slog.Wrap(err)
}

func (d *dataAccess) GetDueNotifications() (map[models.AlertKey]map[string]time.Time, error) {
	conn := d.Get()
	defer conn.Close()
	m, err := redis.Int64Map(conn.Do("ZRANGEBYSCORE", pendingNotificationsKey, 0, time.Now().UTC().Unix(), "WITHSCORES"))
	if err != nil {
		return nil, slog.Wrap(err)
	}
	results := map[models.AlertKey]map[string]time.Time{}
	for key, t := range m {
		last := strings.LastIndex(key, ":")
		if last == -1 {
			continue
		}
		ak, not := models.AlertKey(key[:last]), key[last+1:]
		if results[ak] == nil {
			results[ak] = map[string]time.Time{}
		}
		results[ak][not] = time.Unix(t, 0).UTC()
	}
	return results, err
}

func (d *dataAccess) ClearNotificationsBefore(t time.Time) error {
	conn := d.Get()
	defer conn.Close()

	_, err := conn.Do("ZREMRANGEBYSCORE", pendingNotificationsKey, 0, t.UTC().Unix())
	return slog.Wrap(err)
}

func (d *dataAccess) ClearNotifications(ak models.AlertKey) error {
	conn := d.Get()
	defer conn.Close()

	nots, err := redis.Strings(conn.Do("SMEMBERS", notsByAlertKeyKey(ak)))
	if err != nil {
		return slog.Wrap(err)
	}

	if len(nots) == 0 {
		return nil
	}

	args := []interface{}{pendingNotificationsKey}
	for _, not := range nots {
		key := fmt.Sprintf("%s:%s", ak, not)
		args = append(args, key)
	}
	_, err = conn.Do("ZREM", args...)
	return slog.Wrap(err)
}

func (d *dataAccess) GetNextNotificationTime() (time.Time, error) {
	conn := d.Get()
	defer conn.Close()

	m, err := redis.Int64Map(conn.Do("ZRANGE", pendingNotificationsKey, 0, 0, "WITHSCORES"))
	if err != nil {
		return time.Time{}, slog.Wrap(err)
	}
	// default time is one hour from now if no pending notifications exist
	t := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	for _, i := range m {
		t = time.Unix(i, 0).UTC()
	}
	return t, nil
}
