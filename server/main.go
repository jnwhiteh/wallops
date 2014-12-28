package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

type ServerAPI struct {
	pool connectionPooler
}

// HandleRegister is used to register connections to a given server and
// returns a token that can be used to interact with and subscribe to messages
// from that connection.
func (a *ServerAPI) HandleRegister(w http.ResponseWriter, r *http.Request) {
	var payload RegisterRequest
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Decode the payload
	body, _ := ioutil.ReadAll(r.Body)
	err := json.Unmarshal(body, &payload)
	if err != nil {
		log.Printf("Failed to decode request payload: %s", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// Make sure we don't allow the zero value through
	if !payload.Valid() {
		log.Printf("Invalid request payload: %v", payload)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	token, err := a.pool.Connect(payload.Config)
	if err != nil {
		log.Printf("Failed to connect: %s", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	response := RegisterResponse{
		Success: true,
		Token:   token,
	}
	JSON(w, r, 200, response)
}

// HandleShutdown is the HTTP handler to shut down an existing server
// connection
func (a *ServerAPI) HandleUnregister(w http.ResponseWriter, r *http.Request) {
	var payload TokenRequest

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Decode the payload
	body, _ := ioutil.ReadAll(r.Body)
	err := json.Unmarshal(body, &payload)
	if err != nil {
		log.Printf("Failed to decode request payload: %s", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// Make sure we don't allow the zero value through
	if !payload.Valid() {
		log.Printf("Invalid request payload: %v", payload)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	err = a.pool.Unregister(payload.Token)
	success := err != nil
	response := ErrorResponse{
		Success: success,
		Error:   err.Error(),
	}
	JSON(w, r, 200, response)
}

func main() {
	muxer := http.NewServeMux()
	server := &http.Server{
		Addr:           "localhost:9667",
		Handler:        muxer,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	api := &ServerAPI{
		pool: NewConnectionPool(),
	}

	muxer.HandleFunc("/register", api.HandleRegister)

	log.Printf("Listening on http://%s/", server.Addr)
	log.Fatalln(server.ListenAndServe())
}
