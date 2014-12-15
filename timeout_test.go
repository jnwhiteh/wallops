package main

import (
	"io/ioutil"
	"log"
	"net"
	"testing"

	"github.com/sorcix/irc"
)

type dummyConn struct {
	*net.TCPConn
}

func NewDummyConn() *dummyConn {
	return &dummyConn{
		&net.TCPConn{},
	}
}

type maybeMessage struct {
	message *irc.Message
	err     error
}

// timeoutReader implements the MessageReader interface, but will emulate a
// network timeout for every read operation
type timeoutReader struct {
	queue []maybeMessage
}

func (r *timeoutReader) ReadMessage() (*irc.Message, error) {
	item := r.queue[0]
	r.queue = r.queue[1:]
	return item.message, item.err
}

// timeoutError implements the net.Error interface
type timeoutError struct{}

func (e timeoutError) Error() string {
	return "TimeoutError"
}

func (e timeoutError) Timeout() bool {
	return true
}

func (e timeoutError) Temporary() bool {
	return true
}

// Four read timeouts should result in a message proxy.timeout
func TestReadTimeout(t *testing.T) {
	log.SetOutput(ioutil.Discard)
	timeoutMsg := maybeMessage{nil, timeoutError{}}

	reader := &timeoutReader{
		queue: []maybeMessage{},
	}

	for i := 0; i < missedDeadlineLimit; i++ {
		reader.queue = append(reader.queue, timeoutMsg)
	}

	proxy := Proxy{
		conn:   NewDummyConn(),
		reader: reader,
	}
	timeout := make(chan struct{}, 1)
	proxy.ReadMessages(nil, timeout)

	select {
	case <-timeout:
	default:
		t.Fail()
	}
}

// Ensure that a successful read results in the deadline being extended
func TestDeadlineExtended(t *testing.T) {
	log.SetOutput(ioutil.Discard)
	timeoutMsg := maybeMessage{nil, timeoutError{}}
	validMsg := maybeMessage{&irc.Message{}, nil}

	reader := &timeoutReader{
		queue: []maybeMessage{
			timeoutMsg,
			validMsg,
		},
	}

	for i := 0; i < missedDeadlineLimit; i++ {
		reader.queue = append(reader.queue, timeoutMsg)
	}

	proxy := Proxy{
		conn:   NewDummyConn(),
		reader: reader,
	}

	incoming := make(chan *irc.Message, 1)
	timeout := make(chan struct{}, 1)
	proxy.ReadMessages(incoming, timeout)

	select {
	case <-timeout:
	default:
		t.Fail()
	}
}
