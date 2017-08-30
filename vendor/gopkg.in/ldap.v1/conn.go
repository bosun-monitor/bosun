// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ldap

import (
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net"
	"sync"

	"gopkg.in/asn1-ber.v1"
)

const (
	MessageQuit     = 0
	MessageRequest  = 1
	MessageResponse = 2
	MessageFinish   = 3
)

type messagePacket struct {
	Op        int
	MessageID int64
	Packet    *ber.Packet
	Channel   chan *ber.Packet
}

type sendMessageFlags uint

const (
	startTLS sendMessageFlags = 1 << iota
)

// Conn represents an LDAP Connection
type Conn struct {
	conn                net.Conn
	isTLS               bool
	isClosing           bool
	isStartingTLS       bool
	Debug               debugging
	chanConfirm         chan bool
	chanResults         map[int64]chan *ber.Packet
	chanMessage         chan *messagePacket
	chanMessageID       chan int64
	wgSender            sync.WaitGroup
	wgClose             sync.WaitGroup
	once                sync.Once
	outstandingRequests uint
	messageMutex        sync.Mutex
}

// Dial connects to the given address on the given network using net.Dial
// and then returns a new Conn for the connection.
func Dial(network, addr string) (*Conn, error) {
	c, err := net.Dial(network, addr)
	if err != nil {
		return nil, NewError(ErrorNetwork, err)
	}
	conn := NewConn(c)
	conn.start()
	return conn, nil
}

// DialTLS connects to the given address on the given network using tls.Dial
// and then returns a new Conn for the connection.
func DialTLS(network, addr string, config *tls.Config) (*Conn, error) {
	c, err := tls.Dial(network, addr, config)
	if err != nil {
		return nil, NewError(ErrorNetwork, err)
	}
	conn := NewConn(c)
	conn.isTLS = true
	conn.start()
	return conn, nil
}

// NewConn returns a new Conn using conn for network I/O.
func NewConn(conn net.Conn) *Conn {
	return &Conn{
		conn:          conn,
		chanConfirm:   make(chan bool),
		chanMessageID: make(chan int64),
		chanMessage:   make(chan *messagePacket, 10),
		chanResults:   map[int64]chan *ber.Packet{},
	}
}

func (l *Conn) start() {
	go l.reader()
	go l.processMessages()
	l.wgClose.Add(1)
}

// Close closes the connection.
func (l *Conn) Close() {
	l.once.Do(func() {
		l.isClosing = true
		l.wgSender.Wait()

		l.Debug.Printf("Sending quit message and waiting for confirmation")
		l.chanMessage <- &messagePacket{Op: MessageQuit}
		<-l.chanConfirm
		close(l.chanMessage)

		l.Debug.Printf("Closing network connection")
		if err := l.conn.Close(); err != nil {
			log.Print(err)
		}

		l.wgClose.Done()
	})
	l.wgClose.Wait()
}

// Returns the next available messageID
func (l *Conn) nextMessageID() int64 {
	if l.chanMessageID != nil {
		if messageID, ok := <-l.chanMessageID; ok {
			return messageID
		}
	}
	return 0
}

// StartTLS sends the command to start a TLS session and then creates a new TLS Client
func (l *Conn) StartTLS(config *tls.Config) error {
	messageID := l.nextMessageID()

	if l.isTLS {
		return NewError(ErrorNetwork, errors.New("ldap: already encrypted"))
	}

	packet := ber.Encode(ber.ClassUniversal, ber.TypeConstructed, ber.TagSequence, nil, "LDAP Request")
	packet.AppendChild(ber.NewInteger(ber.ClassUniversal, ber.TypePrimitive, ber.TagInteger, messageID, "MessageID"))
	request := ber.Encode(ber.ClassApplication, ber.TypeConstructed, ApplicationExtendedRequest, nil, "Start TLS")
	request.AppendChild(ber.NewString(ber.ClassContext, ber.TypePrimitive, 0, "1.3.6.1.4.1.1466.20037", "TLS Extended Command"))
	packet.AppendChild(request)
	l.Debug.PrintPacket(packet)

	channel, err := l.sendMessageWithFlags(packet, startTLS)
	if err != nil {
		return err
	}
	if channel == nil {
		return NewError(ErrorNetwork, errors.New("ldap: could not send message"))
	}

	l.Debug.Printf("%d: waiting for response", messageID)
	packet = <-channel
	l.Debug.Printf("%d: got response %p", messageID, packet)
	l.finishMessage(messageID)

	if l.Debug {
		if err := addLDAPDescriptions(packet); err != nil {
			l.Close()
			return err
		}
		ber.PrintPacket(packet)
	}

	if packet.Children[1].Children[0].Value.(int64) == 0 {
		conn := tls.Client(l.conn, config)

		if err := conn.Handshake(); err != nil {
			l.Close()
			return NewError(ErrorNetwork, fmt.Errorf("TLS handshake failed (%v)", err))
		}

		l.isTLS = true
		l.conn = conn
	}
	go l.reader()

	return nil
}

func (l *Conn) sendMessage(packet *ber.Packet) (chan *ber.Packet, error) {
	return l.sendMessageWithFlags(packet, 0)
}

func (l *Conn) sendMessageWithFlags(packet *ber.Packet, flags sendMessageFlags) (chan *ber.Packet, error) {
	if l.isClosing {
		return nil, NewError(ErrorNetwork, errors.New("ldap: connection closed"))
	}
	l.messageMutex.Lock()
	l.Debug.Printf("flags&startTLS = %d", flags&startTLS)
	if l.isStartingTLS {
		l.messageMutex.Unlock()
		return nil, NewError(ErrorNetwork, errors.New("ldap: connection is in startls phase."))
	}
	if flags&startTLS != 0 {
		if l.outstandingRequests != 0 {
			l.messageMutex.Unlock()
			return nil, NewError(ErrorNetwork, errors.New("ldap: cannot StartTLS with outstanding requests"))
		} else {
			l.isStartingTLS = true
		}
	}
	l.outstandingRequests++

	l.messageMutex.Unlock()

	out := make(chan *ber.Packet)
	message := &messagePacket{
		Op:        MessageRequest,
		MessageID: packet.Children[0].Value.(int64),
		Packet:    packet,
		Channel:   out,
	}
	l.sendProcessMessage(message)
	return out, nil
}

func (l *Conn) finishMessage(messageID int64) {
	if l.isClosing {
		return
	}

	l.messageMutex.Lock()
	l.outstandingRequests--
	if l.isStartingTLS {
		l.isStartingTLS = false
	}
	l.messageMutex.Unlock()

	message := &messagePacket{
		Op:        MessageFinish,
		MessageID: messageID,
	}
	l.sendProcessMessage(message)
}

func (l *Conn) sendProcessMessage(message *messagePacket) bool {
	if l.isClosing {
		return false
	}
	l.wgSender.Add(1)
	l.chanMessage <- message
	l.wgSender.Done()
	return true
}

func (l *Conn) processMessages() {
	defer func() {
		if err := recover(); err != nil {
			log.Printf("ldap: recovered panic in processMessages: %v", err)
		}
		for messageID, channel := range l.chanResults {
			l.Debug.Printf("Closing channel for MessageID %d", messageID)
			close(channel)
			delete(l.chanResults, messageID)
		}
		close(l.chanMessageID)
		l.chanConfirm <- true
		close(l.chanConfirm)
	}()

	var messageID int64 = 1
	for {
		select {
		case l.chanMessageID <- messageID:
			messageID++
		case messagePacket, ok := <-l.chanMessage:
			if !ok {
				l.Debug.Printf("Shutting down - message channel is closed")
				return
			}
			switch messagePacket.Op {
			case MessageQuit:
				l.Debug.Printf("Shutting down - quit message received")
				return
			case MessageRequest:
				// Add to message list and write to network
				l.Debug.Printf("Sending message %d", messagePacket.MessageID)
				l.chanResults[messagePacket.MessageID] = messagePacket.Channel
				// go routine
				buf := messagePacket.Packet.Bytes()

				_, err := l.conn.Write(buf)
				if err != nil {
					l.Debug.Printf("Error Sending Message: %s", err.Error())
					break
				}
			case MessageResponse:
				l.Debug.Printf("Receiving message %d", messagePacket.MessageID)
				if chanResult, ok := l.chanResults[messagePacket.MessageID]; ok {
					chanResult <- messagePacket.Packet
				} else {
					log.Printf("Received unexpected message %d", messagePacket.MessageID)
					ber.PrintPacket(messagePacket.Packet)
				}
			case MessageFinish:
				// Remove from message list
				l.Debug.Printf("Finished message %d", messagePacket.MessageID)
				close(l.chanResults[messagePacket.MessageID])
				delete(l.chanResults, messagePacket.MessageID)
			}
		}
	}
}

func (l *Conn) reader() {
	cleanstop := false
	defer func() {
		if err := recover(); err != nil {
			log.Printf("ldap: recovered panic in reader: %v", err)
		}
		if !cleanstop {
			l.Close()
		}
	}()

	for {
		if cleanstop {
			l.Debug.Printf("reader clean stopping (without closing the connection)")
			return
		}
		packet, err := ber.ReadPacket(l.conn)
		if err != nil {
			// A read error is expected here if we are closing the connection...
			if !l.isClosing {
				l.Debug.Printf("reader error: %s", err.Error())
			}
			return
		}
		addLDAPDescriptions(packet)
		if len(packet.Children) == 0 {
			l.Debug.Printf("Received bad ldap packet")
			continue
		}
		l.messageMutex.Lock()
		if l.isStartingTLS {
			cleanstop = true
		}
		l.messageMutex.Unlock()
		message := &messagePacket{
			Op:        MessageResponse,
			MessageID: packet.Children[0].Value.(int64),
			Packet:    packet,
		}
		if !l.sendProcessMessage(message) {
			return
		}

	}
}
