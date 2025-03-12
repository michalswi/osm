package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
)

var tpl = template.Must(template.New("page").Parse(`
<!DOCTYPE html>
<html>
<head>
	<title>osm</title>
	<meta charset="utf-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<link rel="stylesheet" href="https://unpkg.com/leaflet/dist/leaflet.css" />
	<script src="https://unpkg.com/leaflet/dist/leaflet.js"></script>
	<style>
		body { font-family: Arial, sans-serif; text-align: center; }
		#map { height: 70vh; margin-top: 10px; }
		.input-container { margin: 10px; }
		input { padding: 8px; margin: 5px; width: 120px; }
		button { padding: 8px; cursor: pointer; }
	</style>
</head>
<body>
	<h2>Enter Latitude & Longitude</h2>
	<div class="input-container">
		<input type="number" id="lat" placeholder="Latitude" step="any" value="{{.Lat}}">
		<input type="number" id="lon" placeholder="Longitude" step="any" value="{{.Lon}}">
		<button onclick="updateMap()">Show Location</button>
	</div>

	<h2>OR Enter Coordinates (lat,lon)</h2>
	<div class="input-container">
		<input type="text" id="coord" placeholder="e.g. 12.34,56.78">
		<button onclick="updateMapFromText()">Find Location</button>
	</div>

	<h3>Click on the map to get coordinates</h3>

	<div id="map"></div>

	<script>
		var lat = parseFloat("{{.Lat}}");
		var lon = parseFloat("{{.Lon}}");

		var map = L.map('map').setView([lat, lon], 13);

		L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
			attribution: '&copy; OpenStreetMap contributors'
		}).addTo(map);

		var marker = L.marker([lat, lon], { draggable: true }).addTo(map)
			.bindPopup("Current Location")
			.openPopup();

		function updateMap() {
			var newLat = parseFloat(document.getElementById('lat').value);
			var newLon = parseFloat(document.getElementById('lon').value);

			if (!isNaN(newLat) && !isNaN(newLon)) {
				map.setView([newLat, newLon], 13);
				marker.setLatLng([newLat, newLon])
					.bindPopup("New Location: " + newLat + ", " + newLon)
					.openPopup();
			} else {
				alert("Please enter valid latitude and longitude values.");
			}
		}

		function updateMapFromText() {
			var input = document.getElementById('coord').value.trim();
			var parts = input.split(",");

			if (parts.length === 2) {
				var newLat = parseFloat(parts[0]);
				var newLon = parseFloat(parts[1]);

				if (!isNaN(newLat) && !isNaN(newLon)) {
					document.getElementById('lat').value = newLat;
					document.getElementById('lon').value = newLon;
					updateMap();
				} else {
					alert("Invalid format. Use lat,lon (e.g. 12.34,56.78).");
				}
			} else {
				alert("Invalid format. Use lat,lon (e.g. 12.34,56.78).");
			}
		}

		// Click event to get coordinates
		map.on('click', function(e) {
			var clickedLat = e.latlng.lat.toFixed(6);
			var clickedLon = e.latlng.lng.toFixed(6);

			// Update input fields
			document.getElementById('lat').value = clickedLat;
			document.getElementById('lon').value = clickedLon;

			// Move marker to clicked location
			marker.setLatLng([clickedLat, clickedLon])
				.bindPopup("Clicked Location: " + clickedLat + ", " + clickedLon)
				.openPopup();
		});
	</script>
</body>
</html>
`))

func main() {
	var port = "5050"
	http.HandleFunc("/", oms)
	log.Printf("Server started at http://localhost:%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
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
			log.Println("Invalid latitude value:", latParam)
		}
		if parsedLon, err := strconv.ParseFloat(lonParam, 64); err == nil {
			lon = fmt.Sprintf("%f", parsedLon)
		} else {
			log.Println("Invalid longitude value:", lonParam)
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
}
