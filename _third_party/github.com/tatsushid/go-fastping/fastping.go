// go-fastping is a Go language port of Marc Lehmann's AnyEvent::FastPing Perl
// module to send ICMP ECHO REQUEST packets quickly. Original Perl module is
// available at
// http://search.cpan.org/~mlehmann/AnyEvent-FastPing-2.01/
//
// It hasn't been fully implemented original functions yet and only for IPv4
// now.
//
// Here is an example:
//
//	p := fastping.NewPinger()
//	ra, err := net.ResolveIPAddr("ip4:icmp", os.Args[1])
//	if err != nil {
//		fmt.Println(err)
//		os.Exit(1)
//	}
//	p.AddIPAddr(ra)
//	err = p.AddHandler("receive", func(addr *net.IPAddr, rtt time.Duration) {
//		fmt.Printf("IP Addr: %s receive, RTT: %v\n", addr.String(), rtt)
//	})
//	if err != nil {
//		fmt.Println(err)
//		os.Exit(1)
//	}
//	err = p.AddHandler("idle", func() {
//		fmt.Println("finish")
//	})
//	if err != nil {
//		fmt.Println(err)
//		os.Exit(1)
//	}
//	err = p.Run()
//	if err != nil {
//		fmt.Println(err)
//	}
//
// It sends an ICMP packet and wait a response. If it receives a response,
// it calls "receive" callback. After that, MaxRTT time passed, it calls
// "idle" callback. If you need more example, please see "cmd/ping/ping.go".
//
// This library needs to run as a superuser for sending ICMP packets so when
// you run go test, please run as a following
//
//	sudo go test
//
package fastping

import (
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net"
	"sync"
	"syscall"
	"time"
)

func timeToBytes(t time.Time) []byte {
	nsec := t.UnixNano()
	b := make([]byte, 8)
	for i := uint8(0); i < 8; i++ {
		b[i] = byte((nsec >> ((7 - i) * 8)) & 0xff)
	}
	return b
}

func bytesToTime(b []byte) time.Time {
	var nsec int64
	for i := uint8(0); i < 8; i++ {
		nsec += int64(b[i]) << ((7 - i) * 8)
	}
	return time.Unix(nsec/1000000000, nsec%1000000000)
}

type packet struct {
	bytes []byte
	addr  *net.IPAddr
}

type context struct {
	stop chan bool
	done chan bool
	err  error
}

func newContext() *context {
	return &context{
		stop: make(chan bool),
		done: make(chan bool),
	}
}

// Pinger represents ICMP packet sender/receiver
type Pinger struct {
	id  int
	seq int
	// key string is IPAddr.String()
	addrs map[string]*net.IPAddr
	ctx   *context
	mu    sync.Mutex
	// Number of (nano,milli)seconds of an idle timeout. Once it passed,
	// the library calls an idle callback function. It is also used for an
	// interval time of RunLoop() method
	MaxRTT   time.Duration
	handlers map[string]interface{}
	// If Debug is true, it prints debug messages to stdout.
	Debug bool
}

// It returns a new Pinger
func NewPinger() *Pinger {
	rand.Seed(time.Now().UnixNano())
	p := &Pinger{
		id:       rand.Intn(0xffff),
		seq:      rand.Intn(0xffff),
		addrs:    make(map[string]*net.IPAddr),
		MaxRTT:   time.Second,
		handlers: make(map[string]interface{}),
		Debug:    false,
	}
	return p
}

// Add an IP address to Pinger. ipaddr arg should be a string like "192.0.2.1".
func (p *Pinger) AddIP(ipaddr string) error {
	addr := net.ParseIP(ipaddr)
	if addr == nil {
		return errors.New(fmt.Sprintf("%s is not a valid textual representation of an IP address", ipaddr))
	}
	p.mu.Lock()
	p.addrs[addr.String()] = &net.IPAddr{IP: addr}
	p.mu.Unlock()
	return nil
}

// Add an IP address to Pinger. ip arg should be a net.IPAddr pointer.
func (p *Pinger) AddIPAddr(ip *net.IPAddr) {
	p.mu.Lock()
	p.addrs[ip.String()] = ip
	p.mu.Unlock()
}

// Add event handler to Pinger. event arg should be "receive" or "idle" string.
//
// "receive" handler should be
//
//	func(addr *net.IPAddr, rtt time.Duration)
//
// type function. The handler is called with a response packet's source address
// and its elapsed time when Pinger receives a response packet.
//
// "idle" handler should be
//
//	func()
//
// type function. The handler is called when MaxRTT time passed. For more
// detail, please see Run() and RunLoop().
func (p *Pinger) AddHandler(event string, handler interface{}) error {
	switch event {
	case "receive":
		if hdl, ok := handler.(func(*net.IPAddr, time.Duration)); ok {
			p.mu.Lock()
			p.handlers[event] = hdl
			p.mu.Unlock()
			return nil
		} else {
			return errors.New(fmt.Sprintf("Receive event handler should be `func(*net.IPAddr, time.Duration)`"))
		}
	case "idle":
		if hdl, ok := handler.(func()); ok {
			p.mu.Lock()
			p.handlers[event] = hdl
			p.mu.Unlock()
			return nil
		} else {
			return errors.New(fmt.Sprintf("Idle event handler should be `func()`"))
		}
	}
	return errors.New(fmt.Sprintf("No such event: %s", event))
}

// Invoke a single send/receive procedure. It sends packets to all hosts which
// have already been added by AddIP() etc. and wait those responses. When it
// receives a response, it calls "receive" handler registered by AddHander().
// After MaxRTT seconds, it calls "idle" handler and returns to caller with
// an error value. It means it blocks until MaxRTT seconds passed. For the
// purpose of sending/receiving packets over and over, use RunLoop().
func (p *Pinger) Run() error {
	p.mu.Lock()
	p.ctx = newContext()
	p.mu.Unlock()
	p.run(true)
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.ctx.err
}

// Invoke send/receive procedure repeatedly. It sends packets to all hosts which
// have already been added by AddIP() etc. and wait those responses. When it
// receives a response, it calls "receive" handler registered by AddHander().
// After MaxRTT seconds, it calls "idle" handler, resend packets and wait those
// response. MaxRTT works as an interval time.
//
// This is a non-blocking method so immediately returns. If you want to monitor
// and stop sending packets, use Done() and Stop() methods. For example,
//
//	p.RunLoop()
//	ticker := time.NewTicker(time.Millisecond * 250)
//	select {
//	case <-p.Done():
//		if err := p.Err(); err != nil {
//			log.Fatalf("Ping failed: %v", err)
//		}
//	case <-ticker.C:
//		break
//	}
//	ticker.Stop()
//	p.Stop()
//
// For more details, please see "cmd/ping/ping.go".
func (p *Pinger) RunLoop() {
	p.mu.Lock()
	p.ctx = newContext()
	p.mu.Unlock()
	go p.run(false)
}

// Return a channel that is closed when RunLoop() is stopped by an error or
// Stop(). It must be called after RunLoop() call. If not, it causes panic.
func (p *Pinger) Done() <-chan bool {
	return p.ctx.done
}

// Stop RunLoop(). It must be called after RunLoop(). If not, it causes panic.
func (p *Pinger) Stop() {
	p.debugln("Stop(): close(p.ctx.stop)")
	close(p.ctx.stop)
	p.debugln("Stop(): <-p.ctx.done")
	<-p.ctx.done
}

// Return an error that is set by RunLoop(). It must be called after RunLoop().
// If not, it causes panic.
func (p *Pinger) Err() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.ctx.err
}

func (p *Pinger) run(once bool) {
	p.debugln("Run(): Start")
	conn, err := net.ListenIP("ip4:icmp", &net.IPAddr{IP: net.IPv4zero})
	if err != nil {
		p.mu.Lock()
		p.ctx.err = err
		p.mu.Unlock()
		p.debugln("Run(): close(p.ctx.done)")
		close(p.ctx.done)
		return
	}
	defer conn.Close()

	recv := make(chan *packet)
	recvCtx := newContext()

	p.debugln("Run(): call recvICMP4()")
	go p.recvICMP4(conn, recv, recvCtx)

	p.debugln("Run(): call sendICMP4()")
	queue, err := p.sendICMP4(conn)

	ticker := time.NewTicker(p.MaxRTT)

mainloop:
	for {
		select {
		case <-p.ctx.stop:
			p.debugln("Run(): <-p.ctx.stop")
			break mainloop
		case <-recvCtx.done:
			p.debugln("Run(): <-recvCtx.done")
			p.mu.Lock()
			err = recvCtx.err
			p.mu.Unlock()
			break mainloop
		case <-ticker.C:
			p.mu.Lock()
			handler, ok := p.handlers["idle"]
			p.mu.Unlock()
			if ok && handler != nil {
				if hdl, ok := handler.(func()); ok {
					hdl()
				}
			}
			if once || err != nil {
				break mainloop
			}
			p.debugln("Run(): call sendICMP4()")
			queue, err = p.sendICMP4(conn)
		case r := <-recv:
			p.debugln("Run(): <-recv")
			p.procRecv(r, queue)
		}
	}

	ticker.Stop()

	p.debugln("Run(): close(recvCtx.stop)")
	close(recvCtx.stop)
	p.debugln("Run(): <-recvCtx.done")
	<-recvCtx.done

	p.mu.Lock()
	p.ctx.err = err
	p.mu.Unlock()

	p.debugln("Run(): close(p.ctx.done)")
	close(p.ctx.done)
	p.debugln("Run(): End")
}

func (p *Pinger) sendICMP4(conn *net.IPConn) (map[string]*net.IPAddr, error) {
	p.debugln("sendICMP4(): Start")
	p.mu.Lock()
	p.id = rand.Intn(0xffff)
	p.seq = rand.Intn(0xffff)
	p.mu.Unlock()
	queue := make(map[string]*net.IPAddr)
	var wg sync.WaitGroup
	for k, v := range p.addrs {
		p.mu.Lock()
		bytes, err := (&icmpMessage{
			Type: icmpv4EchoRequest, Code: 0,
			Body: &icmpEcho{
				ID: p.id, Seq: p.seq,
				Data: timeToBytes(time.Now()),
			},
		}).Marshal()
		p.mu.Unlock()
		if err != nil {
			wg.Wait()
			return queue, err
		}

		queue[k] = v

		p.debugln("sendICMP4(): Invoke goroutine")
		wg.Add(1)
		go func(ra *net.IPAddr, b []byte) {
			for {
				if _, _, err := conn.WriteMsgIP(bytes, nil, ra); err != nil {
					if neterr, ok := err.(*net.OpError); ok {
						if neterr.Err == syscall.ENOBUFS {
							continue
						}
					}
				}
				break
			}
			p.debugln("sendICMP4(): WriteMsgIP End")
			wg.Done()
		}(v, bytes)
	}
	wg.Wait()
	p.debugln("sendICMP4(): End")
	return queue, nil
}

func (p *Pinger) recvICMP4(conn *net.IPConn, recv chan<- *packet, ctx *context) {
	p.debugln("recvICMP4(): Start")
	for {
		select {
		case <-ctx.stop:
			p.debugln("recvICMP4(): <-ctx.stop")
			close(ctx.done)
			p.debugln("recvICMP4(): close(ctx.done)")
			return
		default:
		}

		bytes := make([]byte, 512)
		conn.SetReadDeadline(time.Now().Add(time.Millisecond * 100))
		p.debugln("recvICMP4(): ReadMsgIP Start")
		_, _, _, ra, err := conn.ReadMsgIP(bytes, nil)
		p.debugln("recvICMP4(): ReadMsgIP End")
		if err != nil {
			if neterr, ok := err.(*net.OpError); ok {
				if neterr.Timeout() {
					p.debugln("recvICMP4(): Read Timeout")
					continue
				} else {
					p.debugln("recvICMP4(): OpError happen", err)
					p.mu.Lock()
					ctx.err = err
					p.mu.Unlock()
					close(ctx.done)
					return
				}
			}
		}
		p.debugln("recvICMP4(): p.recv <- packet")
		recv <- &packet{bytes: bytes, addr: ra}
	}
}

func (p *Pinger) procRecv(recv *packet, queue map[string]*net.IPAddr) {
	addr := recv.addr.String()
	p.mu.Lock()
	if _, ok := p.addrs[addr]; !ok {
		p.mu.Unlock()
		return
	}
	p.mu.Unlock()

	bytes := ipv4Payload(recv.bytes)
	var m *icmpMessage
	var err error
	if m, err = parseICMPMessage(bytes); err != nil {
		return
	}

	if m.Type != icmpv4EchoReply {
		return
	}

	var rtt time.Duration
	switch pkt := m.Body.(type) {
	case *icmpEcho:
		p.mu.Lock()
		if pkt.ID == p.id && pkt.Seq == p.seq {
			rtt = time.Since(bytesToTime(pkt.Data))
		}
		p.mu.Unlock()
	default:
		return
	}

	if _, ok := queue[addr]; ok {
		delete(queue, addr)
		p.mu.Lock()
		handler, ok := p.handlers["receive"]
		p.mu.Unlock()
		if ok && handler != nil {
			if hdl, ok := handler.(func(*net.IPAddr, time.Duration)); ok {
				hdl(recv.addr, rtt)
			}
		}
	}
}

func (p *Pinger) debugln(args ...interface{}) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.Debug {
		log.Println(args...)
	}
}

func (p *Pinger) debugf(format string, args ...interface{}) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.Debug {
		log.Printf(format, args...)
	}
}
