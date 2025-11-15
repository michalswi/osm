package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/michalswi/osm/server"
	"github.com/michalswi/osm/utils"
	"golang.org/x/net/proxy"
)

// This is a simple web server that serves a map page using Leaflet.js, OpenStreetMap and Google Maps.
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

var logMutex sync.Mutex
var logPath string

var ProxyClient *http.Client
var proxyEnabled bool

func initProxy() {
	proxyStr := os.Getenv("PROXY_ADDR")
	proxyEnabled = proxyStr != ""

	if !proxyEnabled {
		log.Println("Proxy disabled - using direct connection")
		ProxyClient = &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				IdleConnTimeout:     90 * time.Second,
				TLSHandshakeTimeout: 10 * time.Second,
				DisableKeepAlives:   false,
			},
		}
		return
	}

	parsed, err := url.Parse(proxyStr)
	if err != nil {
		logger.Fatalf("Invalid PROXY_ADDR: %v", err)
	}

	// SOCKS5 proxy
	if parsed.Scheme == "socks5" {
		var auth *proxy.Auth
		if parsed.User != nil {
			password, _ := parsed.User.Password()
			auth = &proxy.Auth{
				User:     parsed.User.Username(),
				Password: password,
			}
		}

		dialer, err := proxy.SOCKS5("tcp", parsed.Host, auth, proxy.Direct)
		if err != nil {
			logger.Fatalf("SOCKS5 proxy setup failed: %v", err)
		}

		transport := &http.Transport{
			Dial:                dialer.Dial,
			MaxIdleConns:        100,
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
			DisableKeepAlives:   false,
			DisableCompression:  false,
		}

		ProxyClient = &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second,
		}
		logger.Println("SOCKS5 proxy enabled:", proxyStr)
		return
	}

	// HTTP/HTTPS proxy
	transport := &http.Transport{
		Proxy:               http.ProxyURL(parsed),
		MaxIdleConns:        100,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
		DisableKeepAlives:   false,
		DisableCompression:  false,
	}

	ProxyClient = &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}

	logger.Println("HTTP/HTTPS proxy enabled:", proxyStr)
}

func main() {
	initProxy()

	logDir := utils.GetEnv("LOG_DIR", "data")
	logPath = logDirCreation(logDir)

	mux := http.NewServeMux()
	mux.HandleFunc("/", oms)
	mux.HandleFunc("/hz", hz)
	mux.HandleFunc("/robots.txt", robots)

	if proxyEnabled {
		mux.HandleFunc("/proxy/tiles/", proxyTiles)
		mux.HandleFunc("/proxy/nominatim", proxyNominatim)
		logger.Println("Proxy endpoints enabled")
	}

	srv := server.NewServer(mux, port)

	go func() {
		logger.Printf("OSM started on port %s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("ListenAndServe error: %v", err)
		}
	}()

	gracefulShutdown(srv)
}

func logDirCreation(logDir string) string {
	basePath := "/tmp/"
	fullFilePath := filepath.Join(basePath, logDir)
	filepath.Dir(fullFilePath)
	if _, err := os.Stat(fullFilePath); os.IsNotExist(err) {
		err = os.MkdirAll(fullFilePath, 0755)
		if err != nil {
			logger.Fatalf("Log directory creation error: %v", err)
		}
	}
	return fullFilePath
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

	// default coordinates (Wroclaw, Poland)
	lat := "51.109970"
	lon := "17.031984"

	// read locations from file
	locations, err := readLocations()
	if err != nil {
		logger.Printf("Failed to read locations: %v", err)
		locations = []ClientLocation{}
	}

	locationsJSON, err := json.Marshal(locations)
	if err != nil {
		logger.Printf("Failed to marshal locations: %v", err)
		http.Error(w, "Failed to marshal locations", http.StatusInternalServerError)
		return
	}

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

	if proxyEnabled {
		if err := tpl_proxy.Execute(w, data); err != nil {
			http.Error(w, "Internal Error", 500)
		}
		return
	}

	if err := tpl.Execute(w, data); err != nil {
		http.Error(w, "Internal Error", 500)
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

	logMutex.Lock()
	defer logMutex.Unlock()

	var requests []Request
	data, err := os.ReadFile(logPath + "/" + "requests.log")
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Println("Error reading requests.log:", err)
			return
		}
		requests = []Request{}
	} else {
		if len(data) > 0 {
			err = json.Unmarshal(data, &requests)
			if err != nil {
				logger.Println("Error unmarshaling requests.log:", err)
				return
			}
		} else {
			requests = []Request{}
		}
	}

	requests = append(requests, *datas)

	updatedData, err := json.MarshalIndent(requests, "", "    ")
	if err != nil {
		logger.Println("Error marshaling updated requests:", err)
		return
	}

	err = os.WriteFile(logPath+"/"+"requests.log", updatedData, 0644)
	if err != nil {
		logger.Println("Error writing to requests.log:", err)
		return
	}
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

func proxyTiles(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/proxy/tiles/")

	var tileURL string
	if strings.HasPrefix(path, "osm/") {
		// extract z/x/y.png from osm/13/4486/2739.png
		tilePath := strings.TrimPrefix(path, "osm/")
		tileURL = fmt.Sprintf("https://a.tile.openstreetmap.org/%s", tilePath)
	} else if strings.HasPrefix(path, "google/") {
		// extract coordinates and construct Google tiles URL
		tilePath := strings.TrimPrefix(path, "google/")
		// google uses different URL format: lyrs=s&x={x}&y={y}&z={z}
		// parse z/x/y from the path
		parts := strings.Split(strings.TrimSuffix(tilePath, ".png"), "/")
		if len(parts) == 3 {
			tileURL = fmt.Sprintf("https://mt1.google.com/vt/lyrs=s&x=%s&y=%s&z=%s", parts[1], parts[2], parts[0])
		} else {
			http.Error(w, "Invalid Google tile path", http.StatusBadRequest)
			return
		}
	} else if strings.HasPrefix(path, "carto/") {
		// extract z/x/y.png from carto/13/4486/2739.png
		tilePath := strings.TrimPrefix(path, "carto/")
		tileURL = fmt.Sprintf("https://a.basemaps.cartocdn.com/dark_all/%s", tilePath)
	} else {
		http.Error(w, "Invalid tile source", http.StatusBadRequest)
		return
	}

	req, err := http.NewRequest("GET", tileURL, nil)
	if err != nil {
		logger.Printf("Error creating request: %v", err)
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		return
	}

	req.Header.Set("User-Agent", "OSM-Proxy-App/1.0")

	if referer := r.Header.Get("Referer"); referer != "" {
		req.Header.Set("Referer", referer)
	}

	var resp *http.Response
	maxRetries := 3

	for attempt := 1; attempt <= maxRetries; attempt++ {
		resp, err = ProxyClient.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			break
		}

		if err != nil {
			logger.Printf("Attempt %d/%d - Error fetching tile %s: %v", attempt, maxRetries, tileURL, err)
		} else if resp != nil {
			logger.Printf("Attempt %d/%d - Tile server returned status %d for %s", attempt, maxRetries, resp.StatusCode, tileURL)
			resp.Body.Close()
		}

		if attempt < maxRetries {
			time.Sleep(time.Duration(attempt) * 200 * time.Millisecond)
		}
	}

	if err != nil {
		logger.Printf("Failed to fetch tile after %d attempts: %v", maxRetries, err)
		http.Error(w, "Failed to fetch tile", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Printf("Tile server returned status %d for %s", resp.StatusCode, tileURL)
		http.Error(w, "Tile not available", resp.StatusCode)
		return
	}

	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.Header().Set("Cache-Control", "public, max-age=86400")

	if contentLength := resp.Header.Get("Content-Length"); contentLength != "" {
		w.Header().Set("Content-Length", contentLength)
	}

	w.WriteHeader(resp.StatusCode)

	written, err := io.Copy(w, resp.Body)
	if err != nil {
		logger.Printf("Error copying tile response (wrote %d bytes): %v", written, err)
		return
	}

	logRequestDetails(r)
}

func proxyNominatim(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		http.Error(w, "Missing query parameter", http.StatusBadRequest)
		return
	}

	nominatimURL := fmt.Sprintf("https://nominatim.openstreetmap.org/search?format=json&q=%s",
		url.QueryEscape(query))

	resp, err := ProxyClient.Get(nominatimURL)
	if err != nil {
		logger.Printf("Error fetching from Nominatim: %v", err)
		http.Error(w, "Failed to search location", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)

	logRequestDetails(r)
}
