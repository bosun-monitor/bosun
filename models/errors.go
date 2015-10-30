package models

import (
	"time"
)

type AlertError struct {
	FirstTime, LastTime time.Time
	Count               int
	Message             string
}
