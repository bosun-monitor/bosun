package sched

import (
	"fmt"
	"strings"

	"github.com/StackExchange/bosun/conf"
)

func makeFilter(filter string) (func(*conf.Alert, *State) bool, error) {
	fields := strings.Fields(filter)
	if len(fields) == 0 {
		return func(a *conf.Alert, s *State) bool {
			return true
		}, nil
	}
	fs := make(map[string][]func(a *conf.Alert, s *State) bool)
	for _, f := range fields {
		sp := strings.SplitN(f, ":", 2)
		value := sp[len(sp)-1]
		key := sp[0]
		if len(sp) == 1 {
			key = ""
		}
		switch key {
		case "":
			fs[key] = append(fs[key], func(a *conf.Alert, s *State) bool {
				ak := s.AlertKey()
				return strings.Contains(string(ak), value) || strings.Contains(s.Subject, value)
			})
		case "ack":
			var v bool
			switch value {
			case "true":
				v = true
			case "false":
				v = false
			default:
				return nil, fmt.Errorf("unknown %s value: %s", key, value)
			}
			fs[key] = append(fs[key], func(a *conf.Alert, s *State) bool {
				return s.NeedAck != v
			})
		case "notify":
			fs[key] = append(fs[key], func(a *conf.Alert, s *State) bool {
				r := false
				f := func(m map[string]*conf.Notification) {
					for k := range m {
						if strings.Contains(k, value) {
							r = true
							break
						}
					}
				}
				f(a.CritNotification)
				f(a.WarnNotification)
				return r
			})
		case "status":
			var v Status
			switch value {
			case "normal":
				v = StNormal
			case "warning":
				v = StWarning
			case "critical":
				v = StCritical
			case "error":
				v = StError
			case "unknown":
				v = StUnknown
			default:
				return nil, fmt.Errorf("unknown %s value: %s", key, value)
			}
			fs[key] = append(fs[key], func(a *conf.Alert, s *State) bool {
				return s.AbnormalStatus() == v
			})
		default:
			return nil, fmt.Errorf("unknown filter key: %s", key)
		}
	}
	return func(a *conf.Alert, s *State) bool {
		for _, ors := range fs {
			match := false
			for _, f := range ors {
				if f(a, s) {
					match = true
					break
				}
			}
			if !match {
				return false
			}
		}
		return true
	}, nil
}
