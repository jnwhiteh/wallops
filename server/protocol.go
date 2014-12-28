package main

type ServerConfig struct {
	Host     string // the host to connect to
	Port     int    // the port on which to connect
	Password string // a password to be sent to the server
	Nickname string // the nickname to use (if possible)
	Realname string // the name to be displayed in WHOIS queries

	AppName    string // a human-readable application name of registrant
	MessageUrl string // a URL to be called for incoming messages
}

func (c ServerConfig) Valid() bool {
	return (c.Host != "" &&
		c.Port != 0 &&
		c.Nickname != "" &&
		c.Realname != "" &&
		c.AppName != "" &&
		c.MessageUrl != "")
}

type RegisterRequest struct {
	Config ServerConfig // configuration for the server to connect to
}

func (r RegisterRequest) Valid() bool {
	return r.Config.Valid()
}

type RegisterResponse struct {
	Success bool   // whether or not the connection was registered
	Token   string // the token that can be used to access this connection
}

// TokenRequest is a generic payload for any request that requires a server
// token.
type TokenRequest struct {
	Token string // the token for the given connection
}

func (r TokenRequest) Valid() bool {
	return r.Token != ""
}

type ErrorResponse struct {
	Success bool
	Error   string
}
