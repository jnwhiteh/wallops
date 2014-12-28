package main

import (
	"bufio"
	"fmt"
	"net"
	"time"

	"github.com/sorcix/irc"
)

var (
	proxyTimeout = time.Second * 15
)

func NewConnection(config ServerConfig) (*Proxy, error) {
	proxy := &Proxy{config: config}
	err := proxy.Connect()
	if err != nil {
		return nil, err
	}
	return proxy, nil
}

type Proxy struct {
	config ServerConfig

	currentNick string

	conn   net.Conn // the underlying network connection
	reader messageReader
	writer messageWriter
}

func (p *Proxy) formatIncoming(msg interface{}) string {
	color := ""
	colorReset := ""
	return fmt.Sprintf("%s<-- %s%s", color, msg, colorReset)
}

func (p *Proxy) formatOutgoing(msg interface{}) string {
	color := ""
	colorReset := ""
	return fmt.Sprintf("%s--> %s%s", color, msg, colorReset)
}

func (p *Proxy) Connect() error {
	// Make a network connection
	endpoint := fmt.Sprintf("%s:%d", p.config.Host, p.config.Port)
	conn, err := net.Dial("tcp", endpoint)
	if err != nil {
		return err
	}

	// Set a reasonable read deadline for the "connection"
	conn.SetDeadline(time.Now().Add(proxyTimeout))

	// Create IRC protocol encoder/decoders
	encoder := irc.NewEncoder(conn)
	decoder := irc.NewDecoder(bufio.NewReader(conn))
	reader := &safeReader{decoder, p.formatIncoming}
	writer := &writer{encoder, p.formatOutgoing}

	// Send PASS (server password)
	if p.config.Password != "" {
		msg := &irc.Message{
			Command: irc.PASS,
			Params:  []string{p.config.Password}}
		writer.WriteMessage(msg)
		if err != nil {
			return err
		}
	}

	// Send NICK (nickname)
	msg := &irc.Message{Command: irc.NICK, Params: []string{p.config.Nickname}}
	writer.WriteMessage(msg)
	if err != nil {
		return err
	}

	// Send USER (realName and hostmask)
	msg = &irc.Message{
		Command:  irc.USER,
		Params:   []string{p.config.Nickname, "host", "server"},
		Trailing: p.config.Realname,
	}
	err = writer.WriteMessage(msg)
	if err != nil {
		return err
	}

	// Wait for the welcome message and handle nickname in-use responses
	currentNick := p.config.Nickname

	for {
		msg, err := reader.ReadMessage()
		if err != nil {
			return err
		}
		if msg.Command == irc.RPL_WELCOME {
			break
		} else if msg.Command == irc.ERR_NICKNAMEINUSE {
			currentNick = randomNick(p.config.Nickname)
			msg := &irc.Message{Command: irc.NICK, Params: []string{currentNick}}
			err = writer.WriteMessage(msg)
			if err != nil {
				return err
			}
		} else if msg.Command == irc.PING {
			pong := &irc.Message{
				Command: irc.PONG,
				Params:  []string{fmt.Sprintf(":%s", msg.Trailing)},
			}
			err = writer.WriteMessage(pong)
			if err != nil {
				return err
			}
		}
	}

	p.conn = conn
	p.reader = reader
	p.writer = writer
	p.currentNick = currentNick
	return nil
}
