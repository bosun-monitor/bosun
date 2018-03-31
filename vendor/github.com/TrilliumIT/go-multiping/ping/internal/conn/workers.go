package conn

import (
	"context"

	"github.com/TrilliumIT/go-multiping/ping/internal/ping"
)

func (c *Conn) runWorkers(ctx context.Context, workers int, read func() (*ping.Ping, error), handle func(*ping.Ping, error)) {
	for w := 0; w < workers; w++ {
		go c.singleWorker(ctx, read, handle)
	}
}

func ctxDone(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
	}
	return false
}

func (c *Conn) singleWorker(ctx context.Context, read func() (*ping.Ping, error), handle func(*ping.Ping, error)) {
	for {
		p, err := read()
		if ctxDone(ctx) {
			return
		}
		handle(p, err)
	}
}
