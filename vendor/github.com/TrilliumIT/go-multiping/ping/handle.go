package ping

import (
	"github.com/TrilliumIT/go-multiping/ping/internal/ping"
)

// HandleFunc is a function to handle responses and errors
type HandleFunc func(*Ping, error)

func iHandle(handle HandleFunc) func(*ping.Ping, error) {
	return func(p *ping.Ping, err error) { handle(iPingToPing(p), err) }
}
