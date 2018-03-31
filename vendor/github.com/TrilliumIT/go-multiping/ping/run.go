package ping

import (
	"context"
	"time"

	"github.com/TrilliumIT/go-multiping/ping/internal/ping"
)

type ret struct {
	p   *Ping
	err error
}

// takes handle returns a send and a close function and an error
func runOnce(sendGet func(HandleFunc) (func(), func() error, error)) (*Ping, error) {
	rCh := make(chan *ret)
	h := func(p *Ping, err error) {
		rCh <- &ret{p, err}
	}
	send, cClose, err := sendGet(h)
	if err != nil {
		return nil, err
	}
	go send()
	r := <-rCh
	_ = cClose()
	return r.p, r.err
}

func ctxDone(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
	}
	return false
}

func runInterval(ctx context.Context, getPing func() (*ping.Ping, error), sendPing func(*ping.Ping, error), count int, interval time.Duration) {
	var tC <-chan time.Time
	switch interval {
	case 0:
		tc := make(chan time.Time)
		tC = tc
		close(tc)
	default:
		t := time.NewTicker(interval)
		tC = t.C
		defer t.Stop()
	}

	for p, err := getPing(); p.Count < count || count == 0; p, err = getPing() {
		if ctxDone(ctx) {
			return
		}
		sendPing(p, err)
		select {
		case <-ctx.Done():
			return
		case <-tC:
		}
	}
}

func runFlood(ctx context.Context, getPing func() (*ping.Ping, error), sendPing func(*ping.Ping, error), fC <-chan struct{}, count int) {
	for p, err := getPing(); p.Count < count || count == 0; p, err = getPing() {
		if ctxDone(ctx) {
			return
		}
		sendPing(p, err)
		select {
		case <-ctx.Done():
			return
		case <-fC:
		}
	}
}
