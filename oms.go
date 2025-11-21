package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"time"
)

// getCachedLocations returns cached locations if TTL not expired, otherwise reloads from disk.
func getCachedLocations() []ClientLocation {
	locationsCacheMu.RLock()
	fresh := time.Since(locationsCacheStamp) < locationsCacheTTL
	if fresh && locationsCache != nil {
		defer locationsCacheMu.RUnlock()
		return locationsCache
	}
	locationsCacheMu.RUnlock()

	locs, err := readLocations()
	if err != nil {
		logger.Printf("Failed to read locations: %v", err)
		locs = []ClientLocation{}
	}

	locationsCacheMu.Lock()
	locationsCache = locs
	locationsCacheStamp = time.Now()
	locationsCacheMu.Unlock()
	return locs
}

// oms renders the main page (proxy or normal template) with coordinates and location markers.
func oms(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")

	// default coordinates (Wroclaw, Poland)
	lat := "51.109970"
	lon := "17.031984"

	// read locations from file
	locations := getCachedLocations()

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
