package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/sorcix/irc"
)

func Connect(host string, port uint, password, nick, realName string) error {
	// Make a network connection
	endpoint := fmt.Sprintf("%s:%d", host, port)
	conn, err := net.Dial("tcp", endpoint)
	if err != nil {
		return err
	}

	// Set a reasonable read deadline for the "connection"
	conn.SetDeadline(time.Now().Add(time.Second * 30))

	// Create IRC protocol encoder/decoders
	encoder := irc.NewEncoder(conn)
	decoder := irc.NewDecoder(bufio.NewReader(conn))

	// Send PASS (server password)
	if password != "" {
		msg := &irc.Message{Command: irc.PASS, Params: []string{password}}
		err = send(encoder, msg)
		if err != nil {
			return err
		}
	}

	// Send NICK (nickname)
	msg := &irc.Message{Command: irc.NICK, Params: []string{nick}}
	err = send(encoder, msg)
	if err != nil {
		return err
	}

	// Send USER (realName and hostmask)
	msg = &irc.Message{
		Command: irc.USER,
		Params:  []string{nick, "host", "server", realName},
	}
	err = send(encoder, msg)
	if err != nil {
		return err
	}

	// Wait for the welcome message and handle nickname in-use responses
	currentNick := nick

	for {
		msg, err := safeDecode(decoder)
		if err != nil {
			return err
		}
		if msg.Command == irc.RPL_WELCOME {
			break
		} else if msg.Command == irc.ERR_NICKNAMEINUSE {
			currentNick = randomNick(nick)
			msg := &irc.Message{Command: irc.NICK, Params: []string{currentNick}}
			err = send(encoder, msg)
			if err != nil {
				return err
			}
		}
	}

	// At this point we want to be able to relay messages
	//
	// The deadline for a write should be 30 seconds. When a write fails to
	// succeed within that period of time, we should consider the server
	// "dead" and attempt to reconnect.
	//
	// The read deadline should be short enough to maintain a heartbeat
	// between the server and the client. When the timeout has triggered a
	// certain number of times, we should initiate a PING to the server.

	return err
}

func main() {
	host := "irc.snoonet.org"
	host = "localhost"

	err := Connect(host, 6667, "", "bjornbot", "Bjornbot")
	if err != nil {
		log.Print(err)
	}
}
