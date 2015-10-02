package fastping

import (
	"net"
	"sync"
	"testing"
	"time"
)

func TestSource(t *testing.T) {
	for i, tt := range []struct {
		firstAddr  string
		secondAddr string
		invalid    bool
	}{
		{firstAddr: "192.0.2.10", secondAddr: "192.0.2.20", invalid: false},
		{firstAddr: "2001:0DB8::10", secondAddr: "2001:0DB8::20", invalid: false},
		{firstAddr: "192.0.2", invalid: true},
	} {
		p := NewPinger()

		origSource, err := p.Source(tt.firstAddr)
		if tt.invalid {
			if err == nil {
				t.Errorf("[%d] Source should return an error but nothing: %v", i)
			}
			continue
		}
		if err != nil {
			t.Errorf("[%d] Source address failed: %v", i, err)
		}
		if origSource != "" {
			t.Errorf("[%d] Source returned an unexpected value: got %q, expected %q", i, origSource, "")
		}

		origSource, err = p.Source(tt.secondAddr)
		if err != nil {
			t.Errorf("[%d] Source address failed: %v", i, err)
		}
		if origSource != tt.firstAddr {
			t.Errorf("[%d] Source returned an unexpected value: got %q, expected %q", i, origSource, tt.firstAddr)
		}
	}

	v4Addr := "192.0.2.10"
	v6Addr := "2001:0DB8::10"

	p := NewPinger()
	_, err := p.Source(v4Addr)
	if err != nil {
		t.Errorf("Source address failed: %v", err)
	}
	_, err = p.Source(v6Addr)
	if err != nil {
		t.Errorf("Source address failed: %v", err)
	}
	origSource, err := p.Source("")
	if err != nil {
		t.Errorf("Source address failed: %v", err)
	}
	if origSource != v4Addr {
		t.Errorf("Source returned an unexpected value: got %q, expected %q", origSource, v4Addr)
	}
}

func TestAddIP(t *testing.T) {
	addIPTests := []struct {
		host   string
		addr   *net.IPAddr
		expect bool
	}{
		{host: "127.0.0.1", addr: &net.IPAddr{IP: net.IPv4(127, 0, 0, 1)}, expect: true},
		{host: "localhost", addr: &net.IPAddr{IP: net.IPv4(127, 0, 0, 1)}, expect: false},
	}

	p := NewPinger()

	for _, tt := range addIPTests {
		if ok := p.AddIP(tt.host); ok != nil {
			if tt.expect != false {
				t.Errorf("AddIP failed: got %v, expected %v", ok, tt.expect)
			}
		}
	}
	for _, tt := range addIPTests {
		if tt.expect {
			if !p.addrs[tt.host].IP.Equal(tt.addr.IP) {
				t.Errorf("AddIP didn't save IPAddr: %v", tt.host)
			}
		}
	}
}

func TestAddIPAddr(t *testing.T) {
	addIPAddrTests := []*net.IPAddr{
		{IP: net.IPv4(192, 0, 2, 10)},
		{IP: net.IP{0x20, 0x01, 0x0D, 0xB8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x10}},
	}

	p := NewPinger()

	for i, tt := range addIPAddrTests {
		p.AddIPAddr(tt)
		if !p.addrs[tt.String()].IP.Equal(tt.IP) {
			t.Errorf("[%d] AddIPAddr didn't save IPAddr: %v", i, tt.IP)
		}
		if len(tt.IP.To4()) == net.IPv4len {
			if p.hasIPv4 != true {
				t.Errorf("[%d] AddIPAddr didn't save IPAddr type: got %v, expected %v", i, p.hasIPv4, true)
			}
		} else if len(tt.IP) == net.IPv6len {
			if p.hasIPv6 != true {
				t.Errorf("[%d] AddIPAddr didn't save IPAddr type: got %v, expected %v", i, p.hasIPv6, true)
			}
		} else {
			t.Errorf("[%d] AddIPAddr encounted an unexpected error", i)
		}
	}
}

func TestRemoveIP(t *testing.T) {
	p := NewPinger()

	if err := p.AddIP("127.0.0.1"); err != nil {
		t.Fatalf("AddIP failed: %v", err)
	}
	if len(p.addrs) != 1 {
		t.Fatalf("AddIP length check failed")
	}

	if err := p.RemoveIP("127.0"); err == nil {
		t.Fatal("RemoveIP, invalid IP should fail")
	}

	if err := p.RemoveIP("127.0.0.1"); err != nil {
		t.Fatalf("RemoveIP failed: %v", err)
	}
	if len(p.addrs) != 0 {
		t.Fatalf("RemoveIP length check failed")
	}
}

func TestRemoveIPAddr(t *testing.T) {
	p := NewPinger()

	if err := p.AddIP("127.0.0.1"); err != nil {
		t.Fatalf("AddIP failed: %v", err)
	}
	if len(p.addrs) != 1 {
		t.Fatalf("AddIP length check failed")
	}

	p.RemoveIPAddr(&net.IPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if len(p.addrs) != 0 {
		t.Fatalf("RemoveIPAddr length check failed")
	}
}

func TestRun(t *testing.T) {
	for _, network := range []string{"ip", "udp"} {
		p := NewPinger()
		p.Network(network)

		if err := p.AddIP("127.0.0.1"); err != nil {
			t.Fatalf("AddIP failed: %v", err)
		}

		if err := p.AddIP("127.0.0.100"); err != nil {
			t.Fatalf("AddIP failed: %v", err)
		}

		if err := p.AddIP("::1"); err != nil {
			t.Fatalf("AddIP failed: %v", err)
		}

		found1, found100, foundv6 := false, false, false
		called, idle := false, false
		p.OnRecv = func(ip *net.IPAddr, d time.Duration) {
			called = true
			if ip.String() == "127.0.0.1" {
				found1 = true
			} else if ip.String() == "127.0.0.100" {
				found100 = true
			} else if ip.String() == "::1" {
				foundv6 = true
			}
		}

		p.OnIdle = func() {
			idle = true
		}

		err := p.Run()
		if err != nil {
			t.Fatalf("Pinger returns error: %v", err)
		}
		if !called {
			t.Fatalf("Pinger didn't get any responses")
		}
		if !idle {
			t.Fatalf("Pinger didn't call OnIdle function")
		}
		if !found1 {
			t.Fatalf("Pinger `127.0.0.1` didn't respond")
		}
		if found100 {
			t.Fatalf("Pinger `127.0.0.100` responded")
		}
		if !foundv6 {
			t.Fatalf("Pinger `::1` didn't responded")
		}
	}
}

func TestMultiRun(t *testing.T) {
	for _, network := range []string{"ip", "udp"} {
		p1 := NewPinger()
		p1.Network(network)
		p2 := NewPinger()
		p2.Network(network)

		if err := p1.AddIP("127.0.0.1"); err != nil {
			t.Fatalf("AddIP 1 failed: %v", err)
		}

		if err := p2.AddIP("127.0.0.1"); err != nil {
			t.Fatalf("AddIP 2 failed: %v", err)
		}

		var mu sync.Mutex
		res1 := 0
		p1.OnRecv = func(*net.IPAddr, time.Duration) {
			mu.Lock()
			res1++
			mu.Unlock()
		}

		res2 := 0
		p2.OnRecv = func(*net.IPAddr, time.Duration) {
			mu.Lock()
			res2++
			mu.Unlock()
		}

		p1.MaxRTT, p2.MaxRTT = time.Millisecond*100, time.Millisecond*100

		if err := p1.Run(); err != nil {
			t.Fatalf("Pinger 1 returns error: %v", err)
		}
		if res1 == 0 {
			t.Fatalf("Pinger 1 didn't get any responses")
		}
		if res2 > 0 {
			t.Fatalf("Pinger 2 got response")
		}

		res1, res2 = 0, 0
		if err := p2.Run(); err != nil {
			t.Fatalf("Pinger 2 returns error: %v", err)
		}
		if res1 > 0 {
			t.Fatalf("Pinger 1 got response")
		}
		if res2 == 0 {
			t.Fatalf("Pinger 2 didn't get any responses")
		}

		res1, res2 = 0, 0
		errch1, errch2 := make(chan error), make(chan error)
		go func(ch chan error) {
			err := p1.Run()
			if err != nil {
				ch <- err
			}
		}(errch1)
		go func(ch chan error) {
			err := p2.Run()
			if err != nil {
				ch <- err
			}
		}(errch2)
		ticker := time.NewTicker(time.Millisecond * 200)
		select {
		case err := <-errch1:
			t.Fatalf("Pinger 1 returns error: %v", err)
		case err := <-errch2:
			t.Fatalf("Pinger 2 returns error: %v", err)
		case <-ticker.C:
			break
		}
		mu.Lock()
		defer mu.Unlock()
		if res1 != 1 {
			t.Fatalf("Pinger 1 didn't get correct response")
		}
		if res2 != 1 {
			t.Fatalf("Pinger 2 didn't get correct response")
		}
	}
}

func TestRunLoop(t *testing.T) {
	for _, network := range []string{"ip", "udp"} {
		p := NewPinger()
		p.Network(network)

		if err := p.AddIP("127.0.0.1"); err != nil {
			t.Fatalf("AddIP failed: %v", err)
		}
		p.MaxRTT = time.Millisecond * 100

		recvCount, idleCount := 0, 0
		p.OnRecv = func(*net.IPAddr, time.Duration) {
			recvCount++
		}

		p.OnIdle = func() {
			idleCount++
		}

		var err error
		p.RunLoop()
		ticker := time.NewTicker(time.Millisecond * 250)
		select {
		case <-p.Done():
			if err = p.Err(); err != nil {
				t.Fatalf("Pinger returns error %v", err)
			}
		case <-ticker.C:
			break
		}
		ticker.Stop()
		p.Stop()

		if recvCount < 2 {
			t.Fatalf("Pinger receive count less than 2")
		}
		if idleCount < 2 {
			t.Fatalf("Pinger idle count less than 2")
		}
	}
}

func TestErr(t *testing.T) {
	invalidSource := "192.0.2"

	p := NewPinger()
	p.ctx = newContext()

	_ = p.listen("ip4:icmp", invalidSource)
	if p.Err() == nil {
		t.Errorf("Err should return an error but nothing")
	}
}

func TestListen(t *testing.T) {
	noSource := ""
	invalidSource := "192.0.2"

	p := NewPinger()
	p.ctx = newContext()

	conn := p.listen("ip4:icmp", noSource)
	if conn == nil {
		t.Errorf("listen failed: %v", p.Err())
	} else {
		conn.Close()
	}

	conn = p.listen("ip4:icmp", invalidSource)
	if conn != nil {
		t.Errorf("listen should return nothing but something: %v", conn)
		conn.Close()
	}
}

func TestTimeToBytes(t *testing.T) {
	// 2009-11-10 23:00:00 +0000 UTC = 1257894000000000000
	expect := []byte{0x11, 0x74, 0xef, 0xed, 0xab, 0x18, 0x60, 0x00}
	tm, err := time.Parse(time.RFC3339, "2009-11-10T23:00:00Z")
	if err != nil {
		t.Errorf("time.Parse failed: %v", err)
	}
	b := timeToBytes(tm)
	for i := 0; i < 8; i++ {
		if b[i] != expect[i] {
			t.Errorf("timeToBytes failed: got %v, expected: %v", b, expect)
			break
		}
	}
}

func TestBytesToTime(t *testing.T) {
	// 2009-11-10 23:00:00 +0000 UTC = 1257894000000000000
	b := []byte{0x11, 0x74, 0xef, 0xed, 0xab, 0x18, 0x60, 0x00}
	expect, err := time.Parse(time.RFC3339, "2009-11-10T23:00:00Z")
	if err != nil {
		t.Errorf("time.Parse failed: %v", err)
	}
	tm := bytesToTime(b)
	if !tm.Equal(expect) {
		t.Errorf("bytesToTime failed: got %v, expected: %v", tm.UTC(), expect.UTC())
	}
}

func TestTimeToBytesToTime(t *testing.T) {
	tm, err := time.Parse(time.RFC3339, "2009-11-10T23:00:00Z")
	if err != nil {
		t.Errorf("time.Parse failed: %v", err)
	}
	b := timeToBytes(tm)
	tm2 := bytesToTime(b)
	if !tm.Equal(tm2) {
		t.Errorf("bytesToTime failed: got %v, expected: %v", tm2.UTC(), tm.UTC())
	}
}

func TestPayloadSizeDefault(t *testing.T) {
	s := timeToBytes(time.Now())
	d := append(s, byteSliceOfSize(8-TimeSliceLength)...)

	if len(d) != 8 {
		t.Errorf("Payload size incorrect: got %d, expected: %d", len(d), 8)
	}
}

func TestPayloadSizeCustom(t *testing.T) {
	s := timeToBytes(time.Now())
	d := append(s, byteSliceOfSize(64-TimeSliceLength)...)

	if len(d) != 64 {
		t.Errorf("Payload size incorrect: got %d, expected: %d", len(d), 64)
	}
}
