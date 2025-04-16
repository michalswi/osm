package server

import (
	"net/http"
	"time"
)

func NewServer(mux *http.ServeMux, serverPort string) *http.Server {
	srv := &http.Server{
		Addr:         "0.0.0.0:" + serverPort,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	return srv
}
