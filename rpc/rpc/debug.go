package rpc

import "net/http"

var debugLog = false

type debugHTTP struct {
	*Server
}

func (Server *debugHTTP) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	//todo write
}
