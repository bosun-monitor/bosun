package models

import (
	"time"
)

// AlertError is an error model that is used when an alert fails to evaluate
type AlertError struct {
	FirstTime, LastTime time.Time
	Count               int
	Message             string
}
