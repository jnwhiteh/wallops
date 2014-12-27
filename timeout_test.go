package main

import (
	"fmt"
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

// captureWriter captures messages that are written
type captureWriter struct {
	messages []maybeMessage
}

func (w *captureWriter) WriteMessage(msg *irc.Message) error {
	w.messages = append(w.messages, maybeMessage{message: msg})
	return nil
}

func (w *captureWriter) HasPingWaiting() bool {
	if len(w.messages) != 1 {
		return false
	}
	if w.messages[0].message == nil {
		return false
	}
	return w.messages[0].message.Command == irc.PING
}

// Four read timeouts should result in a ping message being sent
func TestReadTimeoutCausesPing(t *testing.T) {
	log.SetOutput(ioutil.Discard)
	timeoutMsg := maybeMessage{nil, timeoutError{}}

	reader := &timeoutReader{}
	writer := &captureWriter{}

	// all reads will timeout
	for i := 0; i < missedDeadlineLimit+1; i++ {
		reader.queue = append(reader.queue, timeoutMsg)
	}

	proxy := Proxy{
		conn:   NewDummyConn(),
		reader: reader,
		writer: writer,
	}
	failure := make(chan error, 1)
	proxy.ReadMessages(nil, failure)

	select {
	case <-failure:
		// verify that a ping message was sent
		if !writer.HasPingWaiting() {
			t.Fatalf("Did not receive a ping message")
		}
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

	for i := 0; i < missedDeadlineLimit+1; i++ {
		reader.queue = append(reader.queue, timeoutMsg)
	}

	writer := &captureWriter{}
	proxy := Proxy{
		conn:   NewDummyConn(),
		reader: reader,
		writer: writer,
	}

	incoming := make(chan *irc.Message, 1)
	failure := make(chan error, 1)
	proxy.ReadMessages(incoming, failure)

	select {
	case <-failure:
		// verify that a ping message was sent
		if !writer.HasPingWaiting() {
			t.Fatalf("Did not receive a ping message")
		}
	default:
		t.Fail()
	}
}

// Unexpected error results in message
func TestUnexpectedError(t *testing.T) {
	log.SetOutput(ioutil.Discard)
	unexpectedError := maybeMessage{nil, fmt.Errorf("Unexpected")}

	reader := &timeoutReader{
		queue: []maybeMessage{unexpectedError},
	}
	proxy := Proxy{
		conn:   NewDummyConn(),
		reader: reader,
	}

	incoming := make(chan *irc.Message, 1)
	failure := make(chan error, 1)
	proxy.ReadMessages(incoming, failure)

	select {
	case err := <-failure:
		if unexpectedError.err != err {
			t.Fatalf("Did not receive expected error")
		}
	default:
		t.Fail()
	}
}
