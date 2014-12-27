package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/sorcix/irc"
)

const proxyTimeout = time.Second * 30
const pongTimeout = time.Second * 15
const missedDeadlineLimit = 5

type ProxyConfig struct {
	host     string
	port     uint
	password string
	nick     string
	realName string
}

func Connect(config ProxyConfig) (*Proxy, error) {
	var proxy *Proxy

	// Make a network connection
	endpoint := fmt.Sprintf("%s:%d", config.host, config.port)
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
	if config.password != "" {
		msg := &irc.Message{Command: irc.PASS, Params: []string{config.password}}
		err = send(encoder, msg)
		if err != nil {
			return proxy, err
		}
	}

	// Send NICK (nickname)
	msg := &irc.Message{Command: irc.NICK, Params: []string{config.nick}}
	err = send(encoder, msg)
	if err != nil {
		return proxy, err
	}

	// Send USER (realName and hostmask)
	msg = &irc.Message{
		Command: irc.USER,
		Params:  []string{config.nick, "host", "server", config.realName},
	}
	err = send(encoder, msg)
	if err != nil {
		return proxy, err
	}

	// Wait for the welcome message and handle nickname in-use responses
	currentNick := config.nick

	for {
		msg, err := safeDecode(decoder)
		if err != nil {
			return proxy, err
		}
		if msg.Command == irc.RPL_WELCOME {
			break
		} else if msg.Command == irc.ERR_NICKNAMEINUSE {
			currentNick = randomNick(config.nick)
			msg := &irc.Message{Command: irc.NICK, Params: []string{currentNick}}
			err = send(encoder, msg)
			if err != nil {
				return proxy, err
			}
		} else if msg.Command == irc.PING {
			pong := &irc.Message{
				Command: irc.PONG,
				Params:  []string{fmt.Sprintf(":%s", msg.Trailing)},
			}
			send(encoder, pong)
		}
	}

	// Build a new proxy object
	proxy = &Proxy{config, endpoint, currentNick, conn, reader, writer}
	return proxy, err
}

// Creates a new proxy object, "reconnects" and transfers everything to the
// existing proxy object.
func (p *Proxy) Reconnect() error {
	newProxy, err := Connect(p.config)
	if err != nil {
		return err
	}

	p.conn = newProxy.conn
	p.reader = newProxy.reader
	p.writer = newProxy.writer
	return nil
}

// Proxy contains the current state of the proxy server
type Proxy struct {
	config ProxyConfig // the configuration of the proxy server

	addr        string // the address of the server the proxy is connected to
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
	failure := make(chan error)
	go p.ReadMessages(incoming, failure)

	for {
		select {
		case msg := <-incoming:
			p.Process(msg)
		case err := <-failure:
			tcpError, ok := err.(net.Error)
			if ok && tcpError.Timeout() {
				log.Printf("Server appears to have timed out!")
			} else {
				log.Printf("Unknown error while reading: %s", err)
			}
			for i := uint(0); i < 300; i++ {
				delay := getExponentialBackoffDelay(i)
				log.Printf("%sWaiting for %v%s", colorWarning, delay, colorReset)
				time.Sleep(delay)
				log.Printf("Attempting to reconnect")
				reconnectError := p.Reconnect()
				if reconnectError != nil {
					log.Printf("Failed to reconnect: %s", reconnectError)
				}

				go p.ReadMessages(incoming, failure)
				break
			}
		}
	}
}

func (p *Proxy) ReadMessages(ch chan<- *irc.Message, failure chan<- error) {
	p.ExtendReadDeadline()

	var waitingForPong string
	skippedDeadlines := 0
	for {
		msg, err := p.reader.ReadMessage()
		if err == nil {
			// Valid message, send to consumer
			ch <- msg
			skippedDeadlines = 0

			if msg.Command == irc.PONG && msg.Trailing == waitingForPong {
				waitingForPong = ""
			} else {
				p.ExtendReadDeadline()
			}

		} else {
			// Is this a timeout error?
			tcpError, ok := err.(net.Error)
			if ok && tcpError.Timeout() {
				log.Printf("%s*** Missed read deadline (%d)%s", colorWarning,
					skippedDeadlines, colorReset)
				skippedDeadlines++

				if waitingForPong != "" {
					// We've timed out without a pong, trigger timeout
					failure <- err
					return

				} else if skippedDeadlines >= missedDeadlineLimit {
					// There's not much more we can do here
					waitingForPong = fmt.Sprintf("%d", time.Now().Nanosecond())
					ping := &irc.Message{
						Command:  irc.PING,
						Trailing: waitingForPong,
					}
					p.Send(ping)

					// Prepare to wait for the pong
					next := time.Now().Add(pongTimeout)
					p.conn.SetReadDeadline(next)
				} else {
					// We have a few more deadlines to go
					p.ExtendReadDeadline()
				}
			} else {
				// Unexpected error
				failure <- err
				return
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
	//	host = "localhost"

	config := ProxyConfig{
		host:     host,
		port:     6667,
		password: "",
		nick:     "bjornbot",
		realName: "Bjornbot",
	}

	proxy, err := Connect(config)
	if err != nil {
		log.Fatal(err)
	}
	proxy.Run()
}
