package models

import (
	"time"
)

type Incident struct {
	Id       uint64
	Start    time.Time
	End      *time.Time
	AlertKey AlertKey
}
