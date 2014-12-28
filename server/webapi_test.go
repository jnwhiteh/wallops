package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
)

type StringReadCloser struct {
	*bytes.Buffer
}

func (src *StringReadCloser) Close() (err error) {
	return
}

func NewStringReadCloser(s string) *StringReadCloser {
	return &StringReadCloser{
		bytes.NewBufferString(s),
	}
}

type NoopConnectionPooler struct {
	calls []ServerConfig
}

func (p *NoopConnectionPooler) Connect(config ServerConfig) (string, error) {
	p.calls = append(p.calls, config)
	return "token", nil
}

func (p *NoopConnectionPooler) Unregister(token string) error {
	return nil
}

func SetupRequest(t *testing.T, method, payload string) (*httptest.ResponseRecorder, *http.Request) {
	w := httptest.NewRecorder()
	buf := NewStringReadCloser(payload)
	r, err := http.NewRequest(method, "http://localhost/register", buf)
	if err != nil {
		t.Fatal(err)
		return nil, nil
	}
	return w, r
}

func TestRegisterInvalidMethod(t *testing.T) {
	api := ServerAPI{NewConnectionPool()}
	w, r := SetupRequest(t, "GET", "{}")

	api.HandleRegister(w, r)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("Did not receive appropriate response")
	}
}

func TestRegisterBadPayload(t *testing.T) {
	api := ServerAPI{NewConnectionPool()}
	w, r := SetupRequest(t, "POST", "{invalid json}")

	api.HandleRegister(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("Did not receive appropriate response")
	}
}

func TestRegisterValidatingConfig(t *testing.T) {
	api := ServerAPI{NewConnectionPool()}

	tests := []string{
		// valid JSON no contents
		`{}`,
		// empty config
		`{"config": {}}`,
	}

	for idx, payload := range tests {
		w, r := SetupRequest(t, "POST", payload)
		api.HandleRegister(w, r)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("Did not receive appropriate for payload %d", idx)
		}
	}
}

// Ensure a valid config (with all required fields) results in a connection
// with the correct server configuration.
func TestRegisterValidConfig(t *testing.T) {
	recordingPool := &NoopConnectionPooler{}
	api := ServerAPI{recordingPool}
	w, r := SetupRequest(t, "POST", `
	{
		"config": {
			"host": "localhost",
			"port": 6667,
			"nickname": "bot",
			"realname": "IRC Bot",
			"appname": "application",
			"messageurl": "http://localhost:9999/"
		}
	}
	`)
	api.HandleRegister(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("Got unexpected non-200 status code")
	}
	if len(recordingPool.calls) != 1 {
		t.Fatalf("Incorrect number of connect calls")
	}
	expectedConfig := ServerConfig{
		Host:       "localhost",
		Port:       6667,
		Nickname:   "bot",
		Realname:   "IRC Bot",
		AppName:    "application",
		MessageUrl: "http://localhost:9999/",
	}
	if recordingPool.calls[0] != expectedConfig {
		t.Fatalf("Got incorrect configuration: %v", recordingPool.calls[0])
	}

	// Validate the response
	body, err := ioutil.ReadAll(w.Body)
	if err != nil {
		t.Fatalf("Failed when reading response body: %s", err)
	}
	expectedResponse := RegisterResponse{
		Success: true,
		Token:   "token",
	}
	jsonValue, _ := json.Marshal(expectedResponse)
	if !bytes.Equal(body, jsonValue) {
		t.Fatalf("Got incorrect response: %s != %s", body, jsonValue)
	}
}
