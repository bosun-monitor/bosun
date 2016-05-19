package goweek

import (
	"errors"
	"time"
)

// A Week is a main package data type
type Week struct {
	Days   []time.Time
	Year   int
	Number int
}

// NewWeek constructs new Week entity from given parameters (year and ISO-8601-compatible week number)
func NewWeek(params ...int) (*Week, error) {
	if len(params) < 2 {
		return &Week{}, errors.New("NewWeek(): too few arguments, specify year and number of week")
	} else if params[0] < 0 {
		return &Week{}, errors.New("NewWeek(): year can't be less than zero")
	} else if params[1] < 1 || params[1] > 53 {
		return &Week{}, errors.New("NewWeek(): number of week can't be less than 1 or greater than 53")
	}

	var (
		week = initWeek(params...)
		day  = 1
		fd   = time.Date(week.Year, 1, day, 0, 0, 0, 0, time.UTC)
		y, w = fd.ISOWeek()
	)

	for y != week.Year && w > 1 {
		day++
		fd = time.Date(week.Year, 1, day, 0, 0, 0, 0, time.UTC)
		y, w = fd.ISOWeek()
	}

	// getting Monday of the 1st week
	for fd.Weekday() > 1 {
		day--
		fd = time.Date(week.Year, 1, day, 0, 0, 0, 0, time.UTC)
	}

	// getting first day of the given week
	var weekNumber = week.Number
	for weekNumber > 1 {
		fd = fd.Add(7 * 24 * time.Hour)
		weekNumber--
	}

	// getting dates for whole week
	for i := 0; i < 7; i++ {
		week.Days = append(week.Days, fd.Add(time.Duration(i)*24*time.Hour))
	}

	return &week, nil
}

// Next calculates and returns information (year, week number and dates) about next week
func (week *Week) Next() (*Week, error) {
	var newYear, newWeek int
	if week.Number+1 > 53 {
		newYear = week.Year + 1
		newWeek = 1
	} else {
		newYear = week.Year
		newWeek = week.Number + 1
	}
	w, e := NewWeek(newYear, newWeek)

	return w, e
}

// Previous calculates and returns information (year, week number and dates) about previous week
func (week *Week) Previous() (*Week, error) {
	var newYear, newWeek int
	if week.Number-1 < 1 {
		newYear = week.Year - 1
		newWeek = 53
	} else {
		newYear = week.Year
		newWeek = week.Number - 1
	}
	w, e := NewWeek(newYear, newWeek)

	return w, e
}

func initWeek(params ...int) Week {
	var week = Week{
		Year:   params[0],
		Number: params[1],
	}
	return week
}
