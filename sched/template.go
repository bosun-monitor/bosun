package sched

import (
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/StackExchange/tsaf/conf"
	"github.com/StackExchange/tsaf/expr"
)

type context struct {
	*State
	Alert *conf.Alert

	schedule *Schedule
}

func (s *Schedule) data(st *State, a *conf.Alert) interface{} {
	return &context{
		State:    st,
		Alert:    a,
		schedule: s,
	}
}

// Ack returns the acknowledge link
func (c *context) Ack() string {
	u := url.URL{
		Scheme: "http",
		Host:   c.schedule.Conf.HttpListen,
		Path:   fmt.Sprintf("/api/acknowledge/%s/%s", c.Alert.Name, c.State.Group.String()),
	}
	if strings.HasPrefix(c.schedule.Conf.HttpListen, ":") {
		h, err := os.Hostname()
		if err != nil {
			return ""
		}
		u.Host = h + u.Host
	}
	return u.String()
}

func (c *context) HostView(host string) string {
	u := url.URL{
		Scheme:   "http",
		Host:     c.schedule.Conf.HttpListen,
		Path:     "/host",
		RawQuery: fmt.Sprintf("time=1d-ago&host=%s", host),
	}
	if strings.HasPrefix(c.schedule.Conf.HttpListen, ":") {
		h, err := os.Hostname()
		if err != nil {
			return ""
		}
		u.Host = h + u.Host
	}
	return u.String()
}

// Format a number (as string) into human readable bytes
type ByteSize float64

const (
	_           = iota // ignore first value by assigning to blank identifier
	KB ByteSize = 1 << (10 * iota)
	MB
	GB
	TB
	PB
	EB
	ZB
	YB
)

func (b ByteSize) String() string {
	switch {
	case b >= YB:
		return fmt.Sprintf("%.2fYB", b/YB)
	case b >= ZB:
		return fmt.Sprintf("%.2fZB", b/ZB)
	case b >= EB:
		return fmt.Sprintf("%.2fEB", b/EB)
	case b >= PB:
		return fmt.Sprintf("%.2fPB", b/PB)
	case b >= TB:
		return fmt.Sprintf("%.2fTB", b/TB)
	case b >= GB:
		return fmt.Sprintf("%.2fGB", b/GB)
	case b >= MB:
		return fmt.Sprintf("%.2fMB", b/MB)
	case b >= KB:
		return fmt.Sprintf("%.2fKB", b/KB)
	}
	return fmt.Sprintf("%.2fB", b)
}

func (c *context) HumanBytes(v string) (s string) {
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return
	}
	b := ByteSize(f)
	s = b.String()
	return
}

func (s *Schedule) ExecuteBody(w io.Writer, a *conf.Alert, st *State) error {
	if a.Template == nil || a.Template.Body == nil {
		return nil
	}
	return a.Template.Body.Execute(w, s.data(st, a))
}

func (s *Schedule) ExecuteSubject(w io.Writer, a *conf.Alert, st *State) error {
	if a.Template == nil || a.Template.Subject == nil {
		return nil
	}
	return a.Template.Subject.Execute(w, s.data(st, a))
}

// E executes the given expression and returns a value with corresponding tags
// to the context's tags. If no such result is found, the first result with nil
// tags is returned. If no such result is found, nil is returned. The precision
// of numbers is truncated for convienent display. Array expressions are not
// supported.
func (c *context) E(v string) (s string) {
	e, err := expr.New(v)
	if err != nil {
		log.Printf("%s: %v", v, err)
		return
	}
	res, err := e.Execute(c.schedule.cache, nil)
	if err != nil {
		log.Printf("%s: %v", v, err)
		return
	}
	for _, r := range res {
		if r.Group.Equal(c.State.Group) {
			s = truncate(r.Value)
		}
	}
	for _, r := range res {
		if r.Group == nil {
			s = truncate(r.Value)
		}
	}
	return
}

// truncate displays needed decimals for a Number.
func truncate(v expr.Value) string {
	switch t := v.(type) {
	case expr.Number:
		if t < 1 {
			return fmt.Sprintf("%.4f", t)
		} else if t < 100 {
			return fmt.Sprintf("%.1f", t)
		} else {
			return fmt.Sprintf("%.0f", t)
		}
	default:
		return ""
	}
}
