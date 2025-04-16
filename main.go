package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/michalswi/osm/server"
)

// This is a simple web server that serves a map page using Leaflet.js and OpenStreetMap.
// It allows users to search for places, enter coordinates, and find their current location.
// The map can be displayed in different styles (street, satellite, dark).

type request struct {
	Timestamp     string `json:"timestamp"`
	Method        string `json:"method"`
	Path          string `json:"path"`
	Query         string `json:"query"`
	UserAgent     string `json:"user_agent"`
	RemoteAddr    string `json:"remote_addr"`
	XForwardedFor string `json:"x_forwarded_for"`
	Referer       string `json:"referer"`
}

func main() {

	mux := http.NewServeMux()
	mux.HandleFunc("/", oms)
	mux.HandleFunc("/hz", hz)
	mux.HandleFunc("/robots.txt", robots)

	srv := server.NewServer(mux, port)

	go func() {
		logger.Printf("OSM started on port %s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("ListenAndServe error: %v", err)
		}
	}()

	gracefulShutdown(srv)
}

func gracefulShutdown(srv *http.Server) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Println("Shutting down server...")
	shutdownCtx, shutdownRelease := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownRelease()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Fatalf("Could not gracefully shutdown the server: %v", err)
	}
	logger.Println("Server stopped.")
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if len(value) == 0 {
		return defaultValue
	}
	return value
}

func hz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
	logRequestDetails(r)
}

func robots(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "./robots.txt")
	logRequestDetails(r)
}

func oms(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "text/html")

	// Default coordinates (Wroclaw, Poland)
	lat := "51.109970"
	lon := "17.031984"

	// Check URL query parameters and validate input (xss)
	if r.URL.Query().Has("lat") && r.URL.Query().Has("lon") {
		latParam := r.URL.Query().Get("lat")
		lonParam := r.URL.Query().Get("lon")
		if parsedLat, err := strconv.ParseFloat(latParam, 64); err == nil {
			lat = fmt.Sprintf("%f", parsedLat)
		} else {
			logger.Println("Invalid latitude value:", latParam)
		}
		if parsedLon, err := strconv.ParseFloat(lonParam, 64); err == nil {
			lon = fmt.Sprintf("%f", parsedLon)
		} else {
			logger.Println("Invalid longitude value:", lonParam)
		}
	}

	data := struct {
		Lat string
		Lon string
	}{
		Lat: lat,
		Lon: lon,
	}

	if err := tpl.Execute(w, data); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}

	logRequestDetails(r)
}

func logRequestDetails(r *http.Request) {
	ua := r.Header.Get("User-Agent")
	ra := r.RemoteAddr
	xforwardedfor := r.Header.Get("X-FORWARDED-FOR")
	if xforwardedfor == "" {
		xforwardedfor = "N/A"
	}
	ref := r.Header.Get("Referer")

	datas := &request{
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
		Method:        r.Method,
		Path:          r.URL.Path,
		Query:         r.URL.RawQuery,
		UserAgent:     ua,
		RemoteAddr:    ra,
		XForwardedFor: xforwardedfor,
		Referer:       ref,
	}

	b, err := json.Marshal(datas)
	if err != nil {
		logger.Println("Error marshalling JSON:", err)
		return
	}
	logger.Printf("%s", b)
}
