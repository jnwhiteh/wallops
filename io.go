package main

import "github.com/sorcix/irc"

// messageReader provides an interface that makes it easy to mock connections
// for testing purposes
type messageReader interface {
	ReadMessage() (*irc.Message, error)
}

type messageWriter interface {
	WriteMessage(*irc.Message) error
}

// safeReader fixes the semantics of the irc package to ensure we can properly
// handle error and invalid messages.
//
// The decoder will ONLY return an error if the underlying bufio.Reader
// returns an error, normally io.EOF. However it may also return a nil
// message, which obviously cannot be used. These cases are converted into a
// parseError return.
type safeReader struct {
	decoder *irc.Decoder
}

func (r *safeReader) ReadMessage() (*irc.Message, error) {
	msg, err := r.decoder.Decode()
	if err != nil {
		return nil, err
	}
	if msg == nil {
		return nil, parseError
	}
	logRecv(msg)
	return msg, err
}

type writer struct {
	encoder *irc.Encoder
}

func (w *writer) WriteMessage(msg *irc.Message) error {
	logSend(msg)
	return w.encoder.Encode(msg)
}
