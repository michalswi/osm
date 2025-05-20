package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/michalswi/osm/server"
)

// This is a simple web server that serves a map page using Leaflet.js and OpenStreetMap.
// It allows users to search for places, enter coordinates, and find their current location.
// The map can be displayed in different styles (street, satellite, dark).

type Request struct {
	Timestamp     string `json:"timestamp"`
	Method        string `json:"method"`
	Path          string `json:"path"`
	Query         string `json:"query"`
	UserAgent     string `json:"user_agent"`
	RemoteAddr    string `json:"remote_addr"`
	XForwardedFor string `json:"x_forwarded_for"`
	Referer       string `json:"referer"`
}

type Location struct {
	Location string `json:"location"`
	As       string `json:"as"`
	Asname   string `json:"asname"`
}

type ClientLocation struct {
	Lat    float64 `json:"lat"`
	Lon    float64 `json:"lon"`
	As     string  `json:"as"`
	Asname string  `json:"asname"`
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

	// Read locations from file
	locations, err := readLocations()
	if err != nil {
		logger.Printf("Failed to read locations: %v", err)
		// Don't fail the request; proceed with default map
		locations = []ClientLocation{}
	}

	// Convert locations to JSON for the template
	locationsJSON, err := json.Marshal(locations)
	if err != nil {
		logger.Printf("Failed to marshal locations: %v", err)
		http.Error(w, "Failed to marshal locations", http.StatusInternalServerError)
		return
	}

	// Check URL query parameters and validate input
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
		Lat           string
		Lon           string
		LocationsJSON template.JS
	}{
		Lat:           lat,
		Lon:           lon,
		LocationsJSON: template.JS(locationsJSON),
	}

	if err := tpl.Execute(w, data); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
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

	datas := &Request{
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

// parseLocationString splits a "latitude,longitude" string into floats
func parseLocationString(locStr string) (lat, lon float64, err error) {
	parts := strings.Split(locStr, ",")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid location format: %s", locStr)
	}

	lat, err = strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid latitude: %s", parts[0])
	}
	if lat < -90 || lat > 90 {
		return 0, 0, fmt.Errorf("latitude out of range: %f", lat)
	}

	lon, err = strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid longitude: %s", parts[1])
	}
	if lon < -180 || lon > 180 {
		return 0, 0, fmt.Errorf("longitude out of range: %f", lon)
	}

	return lat, lon, nil
}

// readLocations reads the locations from the JSON file and converts them to ClientLocation
func readLocations() ([]ClientLocation, error) {
	data, err := os.ReadFile("locations.json")
	if err != nil {
		return nil, err
	}

	var locations []Location
	err = json.Unmarshal(data, &locations)
	if err != nil {
		return nil, err
	}

	var clientLocations []ClientLocation
	for _, loc := range locations {
		lat, lon, err := parseLocationString(loc.Location)
		if err != nil {
			logger.Printf("Skipping invalid location: %v", err)
			continue
		}

		clientLocations = append(clientLocations, ClientLocation{
			Lat:    lat,
			Lon:    lon,
			As:     loc.As,
			Asname: loc.Asname,
		})
	}

	return clientLocations, nil
}
