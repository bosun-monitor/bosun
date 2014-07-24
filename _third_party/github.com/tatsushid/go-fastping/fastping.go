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
//	p.AddHandler("receive", func(addr *net.IPAddr, rtt time.Duration) {
//		fmt.Printf("IP Addr: %s receive, RTT: %v\n", addr.String(), rtt)
//	})
//	p.AddHandler("idle", func() {
//		fmt.Println("finish")
//	})
//	p.Run()
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
	"syscall"
	"time"
)

func init() {
	log.SetFlags(log.Lmicroseconds)
	log.SetPrefix("Debug: ")
}

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

// Pinger represents ICMP packet sender/receiver
type Pinger struct {
	id  int
	seq int
	// key string is IPAddr.String()
	addrs map[string]*net.IPAddr
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
	p.addrs[addr.String()] = &net.IPAddr{IP: addr}
	return nil
}

// Add an IP address to Pinger. ip arg should be a net.IPAddr pointer.
func (p *Pinger) AddIPAddr(ip *net.IPAddr) {
	p.addrs[ip.String()] = ip
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
			p.handlers[event] = hdl
		} else {
			errors.New(fmt.Sprintf("Receive event handler should be `func(*net.IPAddr, time.Duration)`"))
		}
	case "idle":
		if hdl, ok := handler.(func()); ok {
			p.handlers[event] = hdl
		} else {
			errors.New(fmt.Sprintf("Idle event handler should be `func()`"))
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
	return p.run(true, make(chan chan<- bool))
}

// Invode send/receive procedure repeatedly. It sends packets to all hosts which
// have already been added by AddIP() etc. and wait those responses. When it
// receives a response, it calls "receive" handler registered by AddHander().
// After MaxRTT seconds, it calls "idle" handler, resend packets and wait those
// response. MaxRTT works as an interval time.
//
// This is a non-blocking method so immediately returns with channel values.
// If you want to stop sending packets, send a channel value of bool type to it
// and wait for graceful shutdown. For example,
//
//	wait := make(chan bool)
//	quit, errch := p.RunLoop()
//	ticker := time.NewTicker(time.Millisecond * 250)
//	loop:
//	for {
//		select {
//		case err := <-errch:
//			log.Fatalf("Ping failed: %v", err)
//		case <-ticker.C:
//			ticker.Stop()
//			quit <- wait
//		case <-wait:
//			break loop
//		}
//	}
//
// For more detail, please see "cmd/ping/ping.go".
func (p *Pinger) RunLoop() (chan<- chan<- bool, <-chan error) {
	quit := make(chan chan<- bool)
	errch := make(chan error)
	go func(ch chan<- error) {
		err := p.run(false, quit)
		if err != nil {
			ch <- err
		}
	}(errch)
	return quit, errch
}

func (p *Pinger) run(once bool, quit <-chan chan<- bool) error {
	p.debugln("Run(): Start")
	conn, err := net.ListenIP("ip4:icmp", &net.IPAddr{IP: net.IPv4zero})
	if err != nil {
		return err
	}
	defer conn.Close()

	var join chan<- bool
	recv, stoprecv, waitjoin := make(chan *packet), make(chan chan<- bool), make(chan bool)

	p.debugln("Run(): call recvICMP4()")
	go p.recvICMP4(conn, recv, stoprecv)

	p.debugln("Run(): call sendICMP4()")
	queue, err := p.sendICMP4(conn)

	ticker := time.NewTicker(p.MaxRTT)

mainloop:
	for {
		select {
		case join = <-quit:
			p.debugln("Run(): <-quit")
			p.debugln("Run(): stoprecv <- waitjoin")
			stoprecv <- waitjoin
			break mainloop
		case <-ticker.C:
			if handler, ok := p.handlers["idle"]; ok && handler != nil {
				if hdl, ok := handler.(func()); ok {
					hdl()
				}
			}
			if once || err != nil {
				p.debugln("Run(): stoprecv <- waitjoin")
				stoprecv <- waitjoin
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

	p.debugln("Run(): <-waitjoin")
	<-waitjoin
	if !once {
		p.debugln("Run(): join <- true")
		join <- true
	}
	p.debugln("Run(): End")
	return err
}

func (p *Pinger) sendICMP4(conn *net.IPConn) (map[string]*net.IPAddr, error) {
	p.debugln("sendICMP4(): Start")
	p.id = rand.Intn(0xffff)
	p.seq = rand.Intn(0xffff)
	queue := make(map[string]*net.IPAddr)
	qlen := 0
	sent := make(chan bool)
	for k, v := range p.addrs {
		bytes, err := (&icmpMessage{
			Type: icmpv4EchoRequest, Code: 0,
			Body: &icmpEcho{
				ID: p.id, Seq: p.seq,
				Data: timeToBytes(time.Now()),
			},
		}).Marshal()
		if err != nil {
			for i := 0; i < qlen; i++ {
				p.debugln("sendICMP4(): wait goroutine")
				<-sent
				p.debugln("sendICMP4(): join goroutine")
			}
			return queue, err
		}

		queue[k] = v
		qlen++

		p.debugln("sendICMP4(): Invoke goroutine")
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
			sent <- true
		}(v, bytes)
	}
	for i := 0; i < qlen; i++ {
		p.debugln("sendICMP4(): wait goroutine")
		<-sent
		p.debugln("sendICMP4(): join goroutine")
	}
	p.debugln("sendICMP4(): End")
	return queue, nil
}

func (p *Pinger) recvICMP4(conn *net.IPConn, recv chan<- *packet, stoprecv <-chan chan<- bool) {
	p.debugln("recvICMP4(): Start")
	for {
		select {
		case join := <-stoprecv:
			p.debugln("recvICMP4(): <-stoprecv")
			p.debugln("recvICMP4(): join <- true")
			join <- true
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
	if _, ok := p.addrs[addr]; !ok {
		return
	}

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
		if pkt.ID == p.id && pkt.Seq == p.seq {
			rtt = time.Since(bytesToTime(pkt.Data))
		}
	default:
		return
	}

	if _, ok := queue[addr]; ok {
		delete(queue, addr)
		if handler, ok := p.handlers["receive"]; ok {
			if hdl, ok := handler.(func(*net.IPAddr, time.Duration)); ok {
				hdl(recv.addr, rtt)
			}
		}
	}
}

func (p *Pinger) debugln(args ...interface{}) {
	if p.Debug {
		log.Println(args...)
	}
}

func (p *Pinger) debugf(format string, args ...interface{}) {
	if p.Debug {
		log.Printf(format, args...)
	}
}
