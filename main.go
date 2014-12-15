package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/sorcix/irc"
)

const proxyTimeout = time.Minute * 10
const missedDeadlineLimit = 1

func Connect(host string, port uint, password, nick, realName string) (*Proxy, error) {
	var proxy *Proxy

	// Make a network connection
	endpoint := fmt.Sprintf("%s:%d", host, port)
	conn, err := net.Dial("tcp", endpoint)
	if err != nil {
		return proxy, err
	}

	// Set a reasonable read deadline for the "connection"
	conn.SetDeadline(time.Now().Add(proxyTimeout))

	// Create IRC protocol encoder/decoders
	encoder := irc.NewEncoder(conn)
	decoder := irc.NewDecoder(bufio.NewReader(conn))
	reader := &safeReader{decoder}
	writer := &writer{encoder}

	// Send PASS (server password)
	if password != "" {
		msg := &irc.Message{Command: irc.PASS, Params: []string{password}}
		err = send(encoder, msg)
		if err != nil {
			return proxy, err
		}
	}

	// Send NICK (nickname)
	msg := &irc.Message{Command: irc.NICK, Params: []string{nick}}
	err = send(encoder, msg)
	if err != nil {
		return proxy, err
	}

	// Send USER (realName and hostmask)
	msg = &irc.Message{
		Command: irc.USER,
		Params:  []string{nick, "host", "server", realName},
	}
	err = send(encoder, msg)
	if err != nil {
		return proxy, err
	}

	// Wait for the welcome message and handle nickname in-use responses
	currentNick := nick

	for {
		msg, err := safeDecode(decoder)
		if err != nil {
			return proxy, err
		}
		if msg.Command == irc.RPL_WELCOME {
			break
		} else if msg.Command == irc.ERR_NICKNAMEINUSE {
			currentNick = randomNick(nick)
			msg := &irc.Message{Command: irc.NICK, Params: []string{currentNick}}
			err = send(encoder, msg)
			if err != nil {
				return proxy, err
			}
		}
	}

	// Build a new proxy object
	proxy = &Proxy{endpoint, nick, currentNick, conn, reader, writer}
	return proxy, err
}

// Proxy contains the current state of the proxy server
type Proxy struct {
	addr        string // the address of the server the proxy is connected to
	nickname    string // the desired nickname
	currentNick string // the current nickname

	conn   net.Conn // the underlying network connection
	reader messageReader
	writer messageWriter
}

func (p *Proxy) Run() {
	// At this point we want to be able to relay messages
	//
	// The deadline for a write should be 30 seconds. When a write fails to
	// succeed within that period of time, we should consider the server
	// "dead" and attempt to reconnect.
	//
	// The read deadline should be short enough to maintain a heartbeat
	// between the server and the client. When the timeout has triggered a
	// certain number of times, we should initiate a PING to the server.

	incoming := make(chan *irc.Message, 10)
	timeout := make(chan struct{})
	go p.ReadMessages(incoming, timeout)

	for {
		select {
		case msg := <-incoming:
			p.Process(msg)
		case <-timeout:
			log.Printf("Server appears to have timed out!")
		}
	}
}

func (p *Proxy) ReadMessages(ch chan<- *irc.Message, timeout chan<- struct{}) {
	p.ExtendReadDeadline()

	skippedDeadlines := 0
	for {
		msg, err := p.reader.ReadMessage()
		if err == nil {
			// Valid message, send to consumer
			ch <- msg
			p.ExtendReadDeadline()
		} else {
			// Is this a timeout error?
			tcpError, ok := err.(net.Error)
			if ok && tcpError.Timeout() {
				log.Printf("Timeout while reading: %s", err)
				skippedDeadlines++

				if skippedDeadlines >= missedDeadlineLimit {
					// There's not much more we can do here
					timeout <- struct{}{}
					return
				} else {
					// We have a few more deadlines to go
					p.ExtendReadDeadline()
				}
			} else {
				// Unexpected error
				log.Printf("Unknown error while reading: %s", err)
			}
		}
	}
}

func (p *Proxy) ExtendReadDeadline() {
	next := time.Now().Add(proxyTimeout)
	p.conn.SetReadDeadline(next)
}

func (p *Proxy) Send(msg *irc.Message) {
	next := time.Now().Add(proxyTimeout)
	p.conn.SetWriteDeadline(next)
	p.writer.WriteMessage(msg)
}

func (p *Proxy) Process(msg *irc.Message) {
	if msg.Command == irc.PING {
		pong := &irc.Message{
			Command: irc.PONG,
			Params:  []string{fmt.Sprintf(":%s", msg.Trailing)},
		}
		p.Send(pong)
	}
}

func main() {
	host := "irc.snoonet.org"
	host = "localhost"

	proxy, err := Connect(host, 6667, "", "bjornbot", "Bjornbot")
	if err != nil {
		log.Print(err)
	}
	proxy.Run()
}
