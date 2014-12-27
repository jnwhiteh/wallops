package main

import (
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/mgutz/ansi"
	"github.com/sorcix/irc"
)

const maxNickLength = 9

var colorIncoming = ansi.ColorCode("green:black")
var colorOutgoing = ansi.ColorCode("green+bh:black")
var colorWarning = ansi.ColorCode("yellow:black")
var colorReset = ansi.ColorCode("reset")

// In case the IRC package has an issue parsing
var parseError = fmt.Errorf("Failed parsing IRC message")

// randomNick will create a random nickname based on a desired name, with a
// small random bit at the end.
func randomNick(nickname string) string {
	randomBit := rand.Intn(99)
	shortNick := truncate(nickname, maxNickLength-3)
	return fmt.Sprintf("%s_%0d", shortNick, randomBit)
}

func truncate(s string, l int) string {
	if l > len(s) {
		return s
	} else {
		return s[0:l]
	}
}

func logSend(msg *irc.Message) {
	log.Printf("%s--> %s%s", colorOutgoing, msg, colorReset)
}

func logRecv(msg *irc.Message) {
	log.Printf("%s<-- %s%s", colorIncoming, msg, colorReset)
}

func send(encoder *irc.Encoder, msg *irc.Message) error {
	logSend(msg)
	return encoder.Encode(msg)
}

func getExponentialBackoffDelay(attempt uint) time.Duration {
	randomBit := time.Millisecond * time.Duration(rand.Int63n(1001))
	return (time.Second * (1 << (attempt + 1))) + randomBit
}

// safeDecode fixes the semantics of the irc package to ensure we can properly
// handle error and invalid messages.
//
// The decoder will ONLY return an error if the underlying bufio.Reader
// returns an error, normally io.EOF. However it may also return a nil
// message, which obviously cannot be used. These cases are converted into a
// parseError return.
func safeDecode(decoder *irc.Decoder) (*irc.Message, error) {
	msg, err := decoder.Decode()
	if err != nil {
		return nil, err
	}
	if msg == nil {
		return nil, parseError
	}
	logRecv(msg)
	return msg, err
}
