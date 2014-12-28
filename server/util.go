package main

import (
	crand "crypto/rand"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
)

// JSON coerces a value into a JSON response
func JSON(w http.ResponseWriter, r *http.Request, status int, value interface{}) {
	result, err := json.Marshal(value)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	// json rendered fine, write out the result
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(result)
}

// generateToken create a new 16-byte UUID token
func generateToken() (string, error) {
	b := make([]byte, 16)
	_, err := crand.Read(b)
	if err != nil {
		return "", generateTokenError
	}
	return fmt.Sprintf("%x", b), nil
}

var maxNickLength = 9

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
